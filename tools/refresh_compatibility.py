"""Check captured compatibility fixtures against the pinned C++ executable.

Run with --reference EXE --work-dir NEW_SCRATCH. Uses only the recorded inputs;
does not change fixtures or the executable. Requires Windows for this reference.
"""
import argparse
import ctypes
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess


def run(reference, scratch):
    if os.name != "nt":
        raise RuntimeError("the captured reference is a Windows executable")
    ctypes.windll.kernel32.SetErrorMode(0x0001 | 0x0002 | 0x8000)
    project = Path(__file__).resolve().parent.parent
    reference, scratch = Path(reference).resolve(), Path(scratch).resolve()
    # A fresh directory prevents stale outputs from satisfying a failed build.
    scratch.mkdir(parents=True, exist_ok=False)
    expected_hash = hashlib.sha256(reference.read_bytes()).hexdigest()
    reports = []
    for kind, fixture in [("source", project/"assembler/testdata/compatibility.json"),
                          ("cli", project/"internal/cli/testdata/compatibility.json")]:
        report = json.loads(fixture.read_text())
        if report["cpp_sha256"] != expected_hash:
            raise ValueError("reference executable hash does not match the pinned capture")
        for index, case in enumerate(report["cases"]):
            stage = scratch/kind/str(index)
            stage.mkdir(parents=True)
            exe = stage/"reference.exe"
            shutil.copyfile(reference, exe)
            files = case.get("files", {"probe.asm": case.get("source", "")})
            for name, text in files.items():
                path = stage/name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(text, encoding="utf-8")
            if case.get("ini"):
                exe.with_suffix(".ini").write_text(case["ini"], encoding="utf-8")
            args = case.get("args", ["-q", "-i", "probe.asm"])
            proc = subprocess.run([str(exe), *args], cwd=stage, input="\n", text=True,
                                  capture_output=True, timeout=5,
                                  creationflags=subprocess.CREATE_NO_WINDOW)
            outputs = case["outputs"] if kind == "cli" else {"probe.GCT": case["gct_hex"]}
            passed = proc.returncode == 0 and all((stage/name).exists() and (stage/name).read_bytes().hex() == value for name, value in outputs.items())
            reports.append(dict(kind=kind, name=case["name"], passed=passed, exit_code=proc.returncode))
            if not passed:
                raise RuntimeError(f"reference changed: {kind}/{case['name']}: {proc.stdout} {proc.stderr}")
    (scratch/"results.json").write_text(json.dumps(reports, indent=2)+"\n")
    print(f"{len(reports)} captured source/CLI cases reproduced exactly")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reference", required=True)
    parser.add_argument("--work-dir", required=True)
    args = parser.parse_args()
    run(args.reference, args.work_dir)
