"""Capture packaged data-context behavior and verify Go's default-off policies.

No input installation is changed. Pass --reference, --assembler and a fresh
--work-dir. The resulting reference.json can be replayed by the Go test suite.
"""

import argparse
import ctypes
import hashlib
import json
import os
from pathlib import Path
import subprocess


def run(args):
    work = Path(args.work_dir).resolve()
    work.mkdir(parents=True, exist_ok=False)
    assembler, reference = Path(args.assembler).resolve(), Path(args.reference).resolve()
    cases, records, fixtures = [], [], []
    for form in ["CODE @ $80001000", "HOOK @ $80001000", "PULSE", "op"]:
        for body in ["word 2+3", "word x+4", "word x+0x60", "word 0x10+010+-2", "byte 0xfe+1", "half 0xfffe+1",
                     "float NaN", "float 1.0", "double NaN", "double 1.0"]:
            source = "Probe\n.alias x = 0x1000\n"
            source += f"op {body} @ $80001000\n" if form == "op" else f"{form}\n{{\n{body}\n}}\n"
            cases.append((form, body, source))
    for index, (form, body, source) in enumerate(cases):
        path = work / f"probe-{index}.asm"
        path.write_bytes(source.replace("\n", "\r\n").encode("utf-8"))
        floating = body.startswith(("float ", "double "))
        outputs = {}
        for profile, executable, flags in [
            ("packaged", reference, []),
            ("fixes_off", assembler, ["--no-config", "--bug-fixes=false"]),
            ("defaults", assembler, ["--no-config"]),
            ("floating_data", assembler, ["--no-config", "--set=extensions.floating_point_data=true"]),
        ]:
            output = path.with_suffix(".GCT")
            output.unlink(missing_ok=True)
            proc = subprocess.run([str(executable), "-q", "-i", *flags, str(path)], cwd=work,
                                  input=b"\n", capture_output=True, timeout=15)
            diagnostic = (proc.stdout + proc.stderr).decode("utf-8", errors="replace").replace(str(work), "<scratch>")
            outputs[profile] = dict(exit=proc.returncode, hex=output.read_bytes().hex() if output.exists() else None,
                                    diagnostic=diagnostic[-1200:])
        native, compatible, default, extended = [outputs[p] for p in ["packaged", "fixes_off", "defaults", "floating_data"]]
        checks = dict(reference_succeeded=native["exit"] == 0 and native["hex"] is not None,
                      compatibility_identical=compatible["exit"] == 0 and compatible["hex"] == native["hex"],
                      default_policy=default["exit"] == (1 if floating else 0) and
                          (default["hex"] is None if floating else default["hex"] == native["hex"]),
                      extension_policy=extended["exit"] == (1 if form == "op" and body.startswith("double") else 0))
        if floating and not (form == "op" and body.startswith("double")):
            words = {"float NaN": "7fffffff", "float 1.0": "3f800000", "double NaN": "7fffffffffffffff", "double 1.0": "3ff0000000000000"}
            start = 24 if form == "op" else 32  # Hex offset past envelope/write header.
            checks["extended_data"] = extended["hex"] is not None and extended["hex"][start:start+len(words[body])] == words[body]
        records.append(dict(form=form, source=source, outputs=outputs, checks=checks))
        fixtures.append(dict(name=f"{form}/{body}", source=source, gct=native["hex"]))
    digest = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
    report = dict(reference_sha256=digest(reference), assembler_sha256=digest(assembler),
                  cases=records, case_count=len(records), passed=all(all(r["checks"].values()) for r in records))
    (work/"results.json").write_text(json.dumps(report, indent=2)+"\n", encoding="utf-8")
    (work/"reference.json").write_text(json.dumps(dict(reference_sha256=digest(reference), cases=fixtures), indent=2)+"\n", encoding="utf-8")
    print(json.dumps(dict(cases=report["case_count"], passed=report["passed"], failed=[r for r in records if not all(r["checks"].values())]), indent=2))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    if os.name == "nt":
        ctypes.windll.kernel32.SetErrorMode(0x0001 | 0x0002 | 0x8000)
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ["assembler", "reference", "work-dir"]:
        parser.add_argument("--"+name, required=True)
    raise SystemExit(run(parser.parse_args()))
