"""Measure current defaults and independent options against a packaged assembler.

Python 3.11+. Sources are copied into a new scratch directory; no output is
deployed. An optional adapted source tree must already exist. Its differences
from the original manifest are recorded, never counted as untouched compatibility.
The report records failures as findings. Exit 1 means defaults did not match all
six inputs, not that the report could not be produced.
"""

import argparse
import ctypes
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tomllib


ENTRIES = [
    ("RSBE01.txt", "80566528"), ("BOOST.txt", "80550010"),
    ("NETPLAY.txt", "80566528"), ("NETBOOST.txt", "80550010"),
    ("Source/Injects/MDEF.txt", None), ("Source/Injects/DEFINE.txt", None),
]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def comparison(left, right):
    if left is None or right is None:
        return None
    differences = [dict(offset=i, left=left[i:i+4].hex(), right=right[i:i+4].hex())
                   for i in range(0, max(len(left), len(right)), 4)
                   if left[i:i+4] != right[i:i+4]]
    return dict(identical=left == right, left_bytes=len(left), right_bytes=len(right),
                differing_word_positions=len(differences),
                differences=differences if len(left) == len(right) else differences[:16],
                differences_truncated=len(left) != len(right) and len(differences) > 16)


def block(body):
    return "[Probe]\nCODE @ $80001000\n{\n" + body + "\n}\n"


PROBES = {
    "alias-offset": "[Probe]\n.alias x = 0x1000\nCODE @ $80001000\n{\nword x+4\n}\n",
    "literal-add": block("word 2+3"),
    "float-nan": "[Probe]\nfloat NaN @ $80001000\n",
    "double-nan": "[Probe]\ndouble NaN @ $80001000\n",
    "float-block": block("float NaN"),
    "double-block": block("double NaN"),
    "float-block-finite": block("float 1.0"),
    "double-block-finite": block("double 1.0"),
    "mismatched-register": block("fadd r3,r4,r5"),
    "matched-register": block("fadd f3,f4,f5"),
    "numeric-register": block("cmpw r5,0xD"),
    "default-branch-hint": block("bdnz -0x10"),
    "explicit-branch-fix": block("bc+ 13,2,0x10"),
}


class Experiment:
    def __init__(self, args):
        self.args = args
        self.source = Path(args.source_dir).resolve()
        self.work = Path(args.work_dir).resolve()
        self.assembler = Path(args.assembler).resolve()
        self.reference = Path(args.reference).resolve()
        self.config = Path(args.config).resolve()
        self.adapted = Path(args.adapted_source_dir).resolve() if args.adapted_source_dir else None
        for path in [self.source, self.adapted]:
            if path and (self.work == path or self.work.is_relative_to(path)):
                raise ValueError("scratch directory must be outside source trees")
        # Refuse reuse so stale GCTs, configs and sources cannot affect results.
        self.work.mkdir(parents=True, exist_ok=False)
        files = sorted(p for p in (self.source / "Source").rglob("*") if p.is_file())
        files += [self.source / name for name, _ in ENTRIES[:4]]
        self.manifest = {p.relative_to(self.source).as_posix(): sha(p.read_bytes()) for p in files}
        self.replacements = [(str(self.work), "<scratch>"), (str(self.source), "<project-plus>"),
                             (str(self.assembler), "<assembler>"), (str(self.reference), "<reference>"),
                             (str(self.config), "<config>")]
        if self.adapted:
            self.replacements.append((str(self.adapted), "<adapted-source>"))

    def sanitize(self, text):
        for path, replacement in sorted(self.replacements, key=lambda item: -len(item[0])):
            text = text.replace(path, replacement).replace(path.replace("\\", "/"), replacement)
        return text

    def stage(self, name, source=None):
        source = source or self.source
        dest = self.work / name
        for name in self.manifest:
            target = dest / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source / name, target)
            assert sha(target.read_bytes()) == sha((source / name).read_bytes())
        return dest

    def assemble(self, stage, name, base, flags, reference=False):
        path = stage / name
        output = path.with_suffix(".GCT")
        output.unlink(missing_ok=True)
        sidecars = {kind: path.with_name(path.stem + suffix) for kind, suffix in
                    [("text", "_codeset.txt"), ("log", "_log.txt")]}
        for sidecar in sidecars.values():
            sidecar.unlink(missing_ok=True)
        executable = self.reference if reference else self.assembler
        command = [str(executable), "-q", "-i", "-t", "-l", *flags]
        if base:
            command += ["-a", "-b:" + base]
        try:
            process = subprocess.run([*command, str(path)], cwd=stage, input=b"\n",
                                     capture_output=True, timeout=30)
            row = dict(entry=name, exit=process.returncode,
                       diagnostic=self.sanitize((process.stdout + process.stderr).decode(
                           "utf-8", errors="replace"))[-2000:])
        except subprocess.TimeoutExpired:
            row = dict(entry=name, exit=None, timeout=True)
        data = output.read_bytes() if output.exists() else None
        row.update(bytes=len(data) if data is not None else None,
                   sha256=sha(data) if data is not None else None)
        row["sidecars"] = {kind: dict(bytes=p.stat().st_size, sha256=sha(p.read_bytes()))
                           for kind, p in sidecars.items() if p.exists()}
        # A failed build is never treated as comparable output.
        return row, data if row["exit"] == 0 else None

    def builds(self, stage, flags, reference=False):
        rows, outputs = [], {}
        for name, base in ENTRIES:
            row, data = self.assemble(stage, name, base, flags, reference)
            rows.append(row)
            outputs[name] = data
        return dict(flags=[self.sanitize(f) for f in flags], builds=rows), outputs

    def matrix(self, stage, flags):
        baseline, data = self.builds(stage, flags)
        settings = []
        for group, options in tomllib.loads(self.config.read_text(encoding="utf-8")).items():
            if not isinstance(options, dict) or group == "bug_fixes":
                continue
            for key in options:
                setting = group + "." + key
                value = True
                result, outputs = self.builds(stage, flags + [f"--set={setting}={str(value).lower()}"])
                result.update(setting=setting, value=value,
                              comparisons={name: comparison(data[name], outputs[name]) for name, _ in ENTRIES})
                settings.append(result)
                print(stage.name, setting, ",".join(str(r["exit"]) for r in result["builds"]), flush=True)
        return dict(baseline=baseline, settings=settings), data

    def run(self):
        version = subprocess.check_output([str(self.assembler), "--no-config", "--version"], timeout=15).decode("utf-8").strip()
        report = dict(assembler_version=version, run_at_utc=datetime.now(timezone.utc).isoformat(),
                      assembler_sha256=sha(self.assembler.read_bytes()),
                      reference_sha256=sha(self.reference.read_bytes()),
                      config_sha256=sha(self.config.read_bytes()), source_manifest=self.manifest,
                      source_file_count=len(self.manifest), entries=ENTRIES,
                      shared_flags=["-q", "-i", "-t", "-l"],
                      limitations=["Independent toggles, not all combinations.",
                                   "INI disabled: exact_ini_matching is not exercised.",
                                   "No gameplay or hardware execution testing."])
        stage = self.stage("unmodified")
        packaged, native = self.builds(stage, [], reference=True)
        defaults, default_data = self.builds(stage, ["--config", str(self.config)])
        compiled, compiled_data = self.builds(stage, ["--no-config"])
        compatibility, _ = self.builds(stage, ["--no-config", "--bug-fixes=false"])
        report["unmodified"] = dict(packaged=packaged, defaults=defaults, compiled_defaults=compiled,
                                    fixes_off=compatibility,
                                    comparisons={name: comparison(native[name], default_data[name]) for name, _ in ENTRIES},
                                    compiled_and_config_results_agree=defaults["builds"] == compiled["builds"] and default_data == compiled_data)
        matrix, data = self.matrix(stage, ["--no-config", "--bug-fixes=false"])
        matrix["packaged_comparisons"] = {name: comparison(native[name], data[name]) for name, _ in ENTRIES}
        report["compatibility_matches_packaged"] = all(c is not None and c["identical"] for c in matrix["packaged_comparisons"].values())
        report["unmodified_matrix"] = matrix
        if self.adapted:
            stage = self.stage("adapted", self.adapted)
            adapted_manifest = {name: sha((stage/name).read_bytes()) for name in self.manifest}
            packaged, native = self.builds(stage, [], reference=True)
            defaults, _ = self.builds(stage, ["--no-config"])
            matrix, data = self.matrix(stage, ["--no-config"])
            report["adapted"] = dict(packaged=packaged, defaults=defaults, matrix=matrix,
                                     modified_source_hashes={name: value for name, value in adapted_manifest.items() if value != self.manifest[name]},
                                     comparisons={name: comparison(native[name], data[name]) for name, _ in ENTRIES})
            assert all(sha((self.adapted/name).read_bytes()) == value for name, value in adapted_manifest.items())
        stage = self.work / "probes"
        stage.mkdir()
        report["probes"] = []
        for name, source in PROBES.items():
            path = stage / (name + ".asm")
            path.write_bytes(source.replace("\n", "\r\n").encode("utf-8"))
            profiles = [("packaged", [], True), ("defaults", ["--no-config"], False),
                        ("fixes_off", ["--no-config", "--bug-fixes=false"], False)]
            profiles += [(setting, ["--no-config", f"--set={setting}=true"], False) for setting in
                         ["extensions.expression_syntax", "extensions.floating_point_data", "encoding.alternative_float_nan",
                          "encoding.alternative_double_nan", "encoding.gnu_branch_hints",
                          "validation.strict_register_prefixes"]]
            results = []
            for profile, flags, reference in profiles:
                row, data = self.assemble(stage, path.name, None, flags, reference)
                row.update(profile=profile, flags=flags, hex=data.hex() if data else None)
                results.append(row)
            report["probes"].append(dict(name=name, source=source, results=results))
        report["source_unchanged"] = all(sha((self.source/name).read_bytes()) == value for name, value in self.manifest.items())
        report["defaults_match_packaged"] = all(
            row is not None and row["identical"] for row in report["unmodified"]["comparisons"].values())
        (self.work / "results.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        if not report["source_unchanged"]:
            raise RuntimeError("original source hashes changed")
        return 0 if report["defaults_match_packaged"] else 1


if __name__ == "__main__":
    if os.name == "nt":
        ctypes.windll.kernel32.SetErrorMode(0x0001 | 0x0002 | 0x8000)
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ["source-dir", "work-dir", "assembler", "reference", "config"]:
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--adapted-source-dir")
    raise SystemExit(Experiment(parser.parse_args()).run())
