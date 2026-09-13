"""Build unmodified Project+ sources, then compare explicitly adapted builds.

The source installation is read-only. No outputs are deployed and no emulator
profile is used. Run with --source-dir, --assembler, --reference and --work-dir.
"""
import argparse
import ctypes
import difflib
import hashlib
import json
import os
import shutil
import subprocess
from pathlib import Path


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run(args):
    # Suppress Windows crash dialogs from the reference executable; a failed
    # unmodified build is recorded rather than blocking this validation run.
    if os.name == "nt":
        ctypes.windll.kernel32.SetErrorMode(0x0001 | 0x0002 | 0x8000)
    source, work = Path(args.source_dir).resolve(), Path(args.work_dir).resolve()
    if work == source or work.is_relative_to(source):
        raise ValueError("work directory must be outside the source installation")
    work.mkdir(parents=True, exist_ok=True)
    entries = [("RSBE01.txt", "0x80566528"), ("BOOST.txt", "0x80550010"),
               ("NETPLAY.txt", "0x80566528"), ("NETBOOST.txt", "0x80550010"),
               ("Source/Injects/MDEF.txt", None), ("Source/Injects/DEFINE.txt", None)]
    manifest = {str(p.relative_to(source)).replace("\\", "/"): sha(p)
                for p in sorted((source / "Source").rglob("*")) if p.is_file()}
    for name, _ in entries[:4]:
        manifest[name] = sha(source / name)
    reports = []
    repairs = []
    unmodified = []
    for implementation, executable in [("go", args.assembler), ("cpp", args.reference)]:
        stage = work / "unmodified" / implementation
        stage.mkdir(parents=True, exist_ok=True)
        shutil.copytree(source / "Source", stage / "Source", dirs_exist_ok=True)
        for name, _ in entries[:4]:
            shutil.copyfile(source / name, stage / name)
        assert all(sha(stage/name) == value for name, value in manifest.items())
        for entry, base in entries:
            path = stage / entry
            output = path.with_suffix(".GCT")
            output.unlink(missing_ok=True)
            flags = ["-q", "-i"]
            if implementation == "go":
                flags += ["--no-config", "--bug-fixes="+args.bug_fixes, "--set=extensions.dot_op="+args.bug_fixes, "--set=extensions.branch_expressions="+args.bug_fixes, "--set=bug_fixes.console_only=true"] + ["--set="+key+"="+args.bug_fixes for key in ['extensions.expression_syntax', 'extensions.implicit_sections', 'extensions.additional_console_instructions', 'validation.reject_duplicate_labels', 'validation.strict_macro_calls', 'validation.reject_undefined_macros', 'validation.reject_address_annotations', 'validation.reject_data_overflow']]
            if base:
                flags += ["-a", "-b:"+base]
            row = dict(implementation=implementation, entry=entry)
            try:
                proc = subprocess.run([str(Path(executable).resolve()), *flags, str(path)],
                                      cwd=stage, input="\n", text=True, capture_output=True, timeout=15)
                row.update(exit_code=proc.returncode, diagnostic=(proc.stdout+proc.stderr)[-3000:])
            except subprocess.TimeoutExpired:
                row.update(timeout=True, diagnostic="reference exceeded 15 seconds")
            row['output_exists'] = output.exists()
            if output.exists():
                data = output.read_bytes()
                row.update(bytes=len(data), sha256=sha(output), valid_container=len(data)%8==0 and data[:8].hex()=="00d0c0de00d0c0de" and data[-8:].hex()=="f000000000000000")
            unmodified.append(row)
            print('unmodified', implementation, entry, row.get('exit_code','timeout'), flush=True)
    for implementation, executable in [("go", args.assembler), ("cpp", args.reference)]:
        stage = work / implementation
        stage.mkdir(exist_ok=True)
        shutil.copytree(source / "Source", stage / "Source", dirs_exist_ok=True)
        for name, _ in entries[:4]:
            shutil.copyfile(source / name, stage / name)
            path = stage / name
            data = path.read_bytes()
            if b"crclr 6, 6" in data:
                path.write_bytes(data.replace(b"crclr 6, 6",b"crclr 6"))
                repairs.append(dict(implementation=implementation,file=name,change="Remove redundant second operand of crclr", occurrences=data.count(b"crclr 6, 6"),sha256=sha(path)))
        # This distributed source contains an unterminated macro invocation.
        # Correct that exact typo in BOTH staging copies; keep the parser strict
        # and record the repair rather than altering the user's installation.
        macro_file = stage / "Source/Project+/FilePatchCode.asm"
        if macro_file.exists():
            data = macro_file.read_bytes()
            old = b"%LoadAddress(<arg1>,<arg2>\r\n"
            if old in data:
                if data.count(old) != 1:
                    raise ValueError("ambiguous source repair")
                macro_file.write_bytes(data.replace(old,b"%LoadAddress(<arg1>,<arg2>)\r\n"))
                repairs.append(dict(implementation=implementation,file="Source/Project+/FilePatchCode.asm", change="Close unterminated %LoadAddress macro invocation",sha256=sha(macro_file)))
        psa_file = stage / "Source/Community/PSA/PSA.asm"
        if psa_file.exists():
            data = psa_file.read_bytes()
            old = b"HOOK @ $807838B4 8127ee50"
            if old in data:
                if data.count(old) != 1:
                    raise ValueError("ambiguous address annotation repair")
                psa_file.write_bytes(data.replace(old,b"HOOK @ $807838B4 # 8127ee50"))
                repairs.append(dict(implementation=implementation,file="Source/Community/PSA/PSA.asm",change="Comment stray word after hook address",sha256=sha(psa_file)))
        for filename, old, new, reason in [
            ("Source/Project+/Debug/Stage Collisions.asm", b"%setAlpha(notCollidableAlpha)", b"%storeAlpha(notCollidableAlpha)",
             "Use the defined storeAlpha macro for the non-collidable case"),
            ("Source/Project+/MyMusic.asm", b"op addi r27, r27, tlstSongSize @ $8007935A", b"word 0x3b7b0010 @ $8007935A",
             "Preserve the original unaligned write as explicit raw data; its game-level intent needs review"),
            ("Source/ProjectM/Ledge.asm", b"cmpwi r3, 0x14; canSlideOff", b"cmpwi r3, 0x14; bne- canSlideOff",
             "Restore missing branch mnemonic in the two crawl-direction checks; staged interpretation of surrounding logic"),
            ("Source/ProjectM/Modifier/VariableSet.asm", b"loc_0xD8:", b"notFighter:\r\nloc_0xD8:",
             "Define missing notFighter label at the existing register-restoring exit"),
            ("Source/Community/PSA/NewCommands.asm", b"resetAnimInfo:", b"forceCorrection:\r\nresetAnimInfo:",
             "Define missing forceCorrection label at the documented command-28 block"),
            ("Source/Project+/CSE.asm", b"hasFilename:\r\ndidNotFind:", b"hasFilename:",
             "Remove duplicate didNotFind label; retain the default-info block named by the branch comments"),
            ("Source/Project+/Debug/Capsule Renderer.asm", b"fmulls", b"fmuls",
             "Correct misspelled floating multiply mnemonic"),
            ("Source/Project+/Debug/modifiedDebug.asm", b"lmw r0, 0x20(r1)", b"word 0xb8010020",
             "Preserve an invalid overlapping lmw as explicit raw data for comparison only; not hardware-validated"),
            ("Source/Project+/Debug/modifiedDebug.asm", b"addi.", b"addic.",
             "Use addic. for the requested immediate add with CR0 update; staged interpretation"),
            ("Source/Project+/Debug/modifiedDebug.asm", b".op nop", b"op nop",
             "Normalize the .op alias for the C++ reference, which otherwise silently drops these writes"),
            ("Source/Community/ItemEx.asm", b"crclr 6,6", b"crclr 6",
             "Remove redundant second operand of crclr"),
            ("Source/Project+/TexFlags.asm", b"crclr 6, 6", b"crclr 6",
             "Remove redundant second operand of crclr"),
        ]:
            path = stage / filename
            if path.exists() and old in path.read_bytes():
                data = path.read_bytes()
                expected_count = 2 if filename.endswith("Ledge.asm") else 1
                if filename.endswith("Capsule Renderer.asm"): expected_count = 8
                if filename.endswith("modifiedDebug.asm"): expected_count = 2
                if filename.endswith("ItemEx.asm"): expected_count = 3
                if data.count(old)!=expected_count:
                    raise ValueError("ambiguous source adaptation: "+filename)
                path.write_bytes(data.replace(old,new))
                repairs.append(dict(implementation=implementation,file=filename,change=reason,occurrences=expected_count,sha256=sha(path)))
        for entry, base in entries:
            path = stage / entry
            output = path.with_suffix(".GCT")
            output.unlink(missing_ok=True)
            flags = ["-q", "-i", "-l", "-t"]
            if implementation == "go":
                flags += ["--no-config", "--bug-fixes="+args.bug_fixes, "--set=extensions.dot_op="+args.bug_fixes, "--set=extensions.branch_expressions="+args.bug_fixes, "--set=bug_fixes.console_only=true"] + ["--set="+key+"="+args.bug_fixes for key in ['extensions.expression_syntax', 'extensions.implicit_sections', 'extensions.additional_console_instructions', 'validation.reject_duplicate_labels', 'validation.strict_macro_calls', 'validation.reject_undefined_macros', 'validation.reject_address_annotations', 'validation.reject_data_overflow']]
            if base:
                flags += ["-a", "-b:"+base]
            result = subprocess.run([str(Path(executable).resolve()), *flags, str(path)],
                                    cwd=stage, input="\n", text=True, capture_output=True, timeout=60)
            (stage / (path.stem+"-console.txt")).write_text(result.stdout+result.stderr)
            row = dict(implementation=implementation, entry=entry, exit_code=result.returncode,
                       output_exists=output.exists())
            if output.exists():
                data = output.read_bytes()
                row.update(bytes=len(data), sha256=sha(output),
                           valid_container=len(data)%8 == 0 and data[:8].hex()=="00d0c0de00d0c0de" and data[-8:].hex()=="f000000000000000")
            reports.append(row)
            print(implementation, entry, result.returncode, row.get("bytes"), flush=True)
            if result.returncode or not output.exists():
                print((result.stdout+result.stderr)[-2500:], flush=True)
    comparisons = []
    categories={}
    for entry, _ in entries:
        go, cpp = [(work / implementation / entry).with_suffix(".GCT") for implementation in ("go", "cpp")]
        if not go.exists() or not cpp.exists():
            continue
        a, b = go.read_bytes(), cpp.read_bytes()
        differing = [i for i in range(0,min(len(a),len(b)),4) if a[i:i+4]!=b[i:i+4]]
        changes=[]
        for i in differing:
            g,c=int.from_bytes(a[i:i+4],'big'),int.from_bytes(b[i:i+4],'big')
            reason='unexplained'
            if g^c==0x80000000: reason='full-width directive address'
            elif g^c==0x00200000 and g>>26==16: reason='default branch prediction'
            elif (g,c)==(0x7fc00000,0x7fffffff):reason='canonical quiet-NaN payload'
            elif g>>26==42 and c>>26==40 and g&0x3ffffff==c&0x3ffffff:reason='lha opcode correction'
            elif (g,c)==(0x7c056800,0x7c050000):reason='hexadecimal register number (GNU verified)'
            elif (g,c)==(0x54001838,0x54000000):reason='hexadecimal rotate fields (GNU verified)'
            categories[reason]=categories.get(reason,0)+1
            changes.append(dict(offset=i,go=f'{g:08x}',cpp=f'{c:08x}',reason=reason))
        comparisons.append(dict(entry=entry, identical=a==b, go_bytes=len(a), cpp_bytes=len(b),
                                differing_word_count=len(differing),
                                differences=changes))
    unmodified_comparisons = []
    for entry, _ in entries:
        go, cpp = [next(row for row in unmodified
                        if row['entry'] == entry and row['implementation'] == implementation)
                   for implementation in ('go', 'cpp')]
        identical = (go.get('exit_code') == 0 and cpp.get('exit_code') == 0
                     and go.get('valid_container') and cpp.get('valid_container')
                     and go.get('sha256') == cpp.get('sha256'))
        unmodified_comparisons.append(dict(entry=entry, identical=bool(identical)))
    report = dict(bug_fixes=args.bug_fixes == "true", dot_op=args.bug_fixes == "true", branch_expressions=args.bug_fixes == "true", allow_non_console_instructions=False, source_dir=str(source), source_manifest=manifest,
                  assembler_sha256=sha(Path(args.assembler)), reference_sha256=sha(Path(args.reference)),
                  unmodified_builds=unmodified, unmodified_comparisons=unmodified_comparisons,
                  builds=reports, comparisons=comparisons, staging_repairs=repairs)
    report['difference_categories']=categories
    report['comparison_passed']=len(comparisons)==6 and all(r['go_bytes']==r['cpp_bytes'] for r in comparisons) and not categories.get('unexplained')
    if args.bug_fixes == 'false':
        report['comparison_passed'] = (report['comparison_passed']
                                       and all(r['identical'] for r in comparisons)
                                       and all(r['identical'] for r in unmodified_comparisons))
    patch=[]
    for filename in sorted({r['file'] for r in repairs}):
        before=(source/filename).read_text(errors='replace').splitlines(keepends=True)
        after=(work/'go'/filename).read_text(errors='replace').splitlines(keepends=True)
        patch.extend(difflib.unified_diff(before,after,fromfile='a/'+filename,tofile='b/'+filename))
    (work/'staging-source.patch').write_text(''.join(patch))
    report['source_unchanged']=all(sha(source/name)==value for name,value in manifest.items())
    (work / "results.json").write_text(json.dumps(report,indent=2)+"\n")
    return 0 if report['source_unchanged'] and report['comparison_passed'] and len(reports)==12 and all(r["exit_code"]==0 and r.get("valid_container") for r in reports) else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("source-dir", "assembler", "reference", "work-dir"):
        parser.add_argument("--"+name,required=True)
    parser.add_argument("--bug-fixes", choices=("true", "false"), default="false")
    raise SystemExit(run(parser.parse_args()))
