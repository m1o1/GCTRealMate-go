"""Execute Go-assembled instruction checks in an isolated Dolphin instance.

Uses only Python's standard library. No game, BIOS, or existing Dolphin profile
is used. Pass --dolphin, --assembler, and --work-dir explicitly. On Windows the
child is started hidden; its batch/Null backend renders no game graphics.
"""
import argparse
import hashlib
import json
import socket
import struct
import subprocess
import time
from pathlib import Path


class Remote:
    def __init__(self, sock):
        self.sock = sock
        sock.settimeout(10)

    def read(self):
        while True:
            c = self.sock.recv(1)
            if not c:
                raise RuntimeError("Dolphin closed debugger connection")
            if c == b"$":
                break
        payload = bytearray()
        while (c := self.sock.recv(1)) != b"#":
            if not c:
                raise RuntimeError("truncated debugger packet")
            payload.extend(c)
        checksum = self.sock.recv(1) + self.sock.recv(1)
        if int(checksum, 16) != sum(payload) & 255:
            raise RuntimeError("debugger checksum mismatch")
        self.sock.sendall(b"+")
        if not payload:
            return self.read()
        return payload.decode()

    def send(self, text):
        b = text.encode()
        self.sock.sendall(b"$" + b + b"#" + f"{sum(b)&255:02x}".encode())

    def command(self, text):
        self.send(text)
        reply = self.read()
        # A breakpoint notification can be queued independently of a reply.
        while text not in ("?", "c", "s") and reply.startswith(("T", "S")):
            reply = self.read()
        return reply


def program():
    code = ["lis r30,0x8001"]
    expected = []

    def emit(*lines):
        code.extend(lines)

    def check(name, want):
        emit(f"stw r3,{4*len(expected)}(r30)")
        expected.append((name, want & 0xffffffff))

    emit("li r4,-1", "sth r4,4096(r30)", "lha r3,4096(r30)")
    check("lha sign extension", -1)
    emit("lhz r3,4096(r30)")
    check("lhz zero extension", 65535)
    emit("li r4,0x1234", "li r5,0x5678", "eqv r3,r4,r5")
    check("eqv distinct operands", ~(0x1234 ^ 0x5678))
    emit("lis r4,0x4000", "mtcr r4", "crandc 0,1,2", "mfcr r3")
    check("crandc complement", 0xc0000000)
    emit("lis r4,0x2000", "mtcr r4", "crorc 0,1,2", "mfcr r3")
    check("crorc complement", 0x20000000)
    emit("li r0,0", "mtxer r0", "lis r4,0x7fff", "ori r4,r4,0xffff", "li r5,1", "addo r3,r4,r5")
    check("addo result", 0x80000000)
    emit("mfxer r3")
    check("addo overflow flags", 0xc0000000)
    emit("li r4,0x1234", "li r3,0", "srwi r3,r4,0")
    check("srwi zero destination", 0x1234)
    emit("li r3,0", "li r4,7", "mtctr r4", "loop:", "addi r3,r3,1", "bdnz loop")
    check("backward counted branch", 7)
    emit("cmpwi r3,7", "beq equal", "li r3,-1", "b done_compare", "equal:", "li r3,42", "done_compare:")
    check("conditional label branch", 42)
    emit("bl function", "b after_function", "function:", "addi r3,r3,1", "blr", "after_function:")
    check("branch link and return", 43)
    emit("lis r3,0xa000", "mtspr 920,r3", "li r3,0", "mtspr 912,r3")
    for offset, value in enumerate([0x3fc00000, 0x40100000, 0x40800000, 0x41000000]):
        emit(f"lis r3,{value>>16}", f"stw r3,{4096+offset*4}(r30)")
    emit("addi r29,r30,4112", "psq_l f1,-16(r29),0,0", "li r4,-8", "psq_lx f2,r29,r4,0,0")

    def pair(name, op, a, b):
        emit(op, f"psq_st f3,{4*len(expected)}(r30),0,0")
        for label, value in [("PS0", a), ("PS1", b)]:
            expected.append((name+" "+label, struct.unpack(">I", struct.pack(">f", value))[0]))

    pair("paired add with negative/indexed loads", "ps_add f3,f1,f2", 5.5, 10.25)
    pair("paired multiply", "ps_mul f3,f1,f2", 6.0, 18.0)
    pair("paired multiply-add", "ps_madd f3,f1,f2,f1", 7.5, 20.25)
    emit("lis r3,0x0f00", "mtcr r3", "ps_add. f3,f1,f2", "mfcr r3", "andis. r3,r3,0x0f00")
    check("paired record updates CR1", 0)
    emit("fadds f3,f1,f2", f"stfs f3,{4*len(expected)}(r30)")
    expected.append(("scalar floating add", 0x40b00000))
    emit("lis r3,4", "ori r3,r3,4", "mtspr 913,r3", "li r3,1", "stb r3,-16(r29)", "li r3,2", "stb r3,-15(r29)")
    emit("psq_l f3,-16(r29),0,1", f"psq_st f3,{4*len(expected)}(r30),0,0")
    expected.extend([("quantized byte load PS0", 0x3f800000), ("quantized byte load PS1", 0x40000000)])
    emit("lis r4,0x1234", "ori r4,r4,0x5678", "mtcr r4", "mcrf cr5,cr2", "mfcr r3")
    check("mcrf field copy", 0x12345378)
    emit("li r0,0", "mtcr r0", "lis r4,0xe000", "mtxer r4", "mcrxr cr4", "mfcr r3")
    check("mcrxr copies XER flags", 0x0000e000)
    emit("mfxer r3")
    check("mcrxr clears XER flags", 0)
    emit("mfmsr r6", "mtmsr r6", "isync", "mfmsr r3", "xor r3,r3,r6")
    check("MSR read/write round trip", 0)
    emit("mfsr r6,3", "lis r4,0x12", "ori r4,r4,0x3456", "mtsr 3,r4", "mfsr r3,3")
    check("segment register direct read/write", 0x123456)
    emit("lis r7,0x3000", "mfsrin r3,r7")
    check("segment register indexed read", 0x123456)
    emit("mtsrin r6,r7", "mfsr r3,3", "xor r3,r3,r6")
    check("segment register indexed restore", 0)
    emit("mftbu r6", "mftb r3,269", "xor r3,r3,r6")
    check("time-base upper selectors agree", 0)
    emit("mftb r6", "mftbl r3", "cmplw r3,r6", "li r3,0", "bge time_ok", "li r3,1", "time_ok:")
    check("time-base lower advances", 0)

    def fpscr(name, want):
        emit("mffs f3", "stfd f3,4160(r30)", "lwz r3,4164(r30)", "andi. r3,r3,15")
        check(name,want)

    emit("mtfsfi 7,0", "mffs f4", "mtfsfi. 7,3")
    fpscr("mtfsfi sets rounding bits", 3)
    emit("mtfsb0. 31")
    fpscr("mtfsb0 clears FPSCR bit", 2)
    emit("mtfsb1. 31")
    fpscr("mtfsb1 sets FPSCR bit", 3)
    emit("li r0,0", "mtcr r0", "mcrfs cr5,cr7", "mfcr r3")
    check("mcrfs copies FPSCR field", 0x00000300)
    emit("mtfsf. 255,f4")
    fpscr("mtfsf restores FPSCR",0)
    emit("li r0,0", "tlbie r0", "tlbsync", "sync", "li r3,1")
    check("TLB invalidate/synchronize returns",1)
    emit("mfspr r6,hid0", "ori r6,r6,0x4000", "mtspr hid0,r6", "isync")
    emit("lis r6,0xb000", "mtspr hid2,r6", "addi r4,r30,4224", "li r5,1", "stw r5,0(r4)", "dcbf r0,r4", "dcbz_l r0,r4", "lwz r3,0(r4)")
    check("dcbz_l clears cache line in Dolphin",0)
    emit("mfspr r6,ear", "lis r7,0x8000", "mtspr ear,r7", "addi r4,r30,4256", "li r5,0x3456",
         "ecowx r5,r0,r4", "eciwx r3,r0,r4", "mtspr ear,r6")
    check("external-control word read/write",0x3456)
    emit("halt:", "b halt")
    return code, expected


def validate_handler(remote, args, work, halt, mem2=False):
    """Run Dolphin's actual bundled Gecko handler over generated GCT bytes."""
    statements = ["Gecko handler validation"]
    expected = []
    def write(statement, address, data):
        statements.append(statement)
        expected.append((statement, address, bytes.fromhex(data)))
    write("word 0x89abcdef @ $80020000", 0x80020000, "89abcdef")
    write("byte 0xab @ $80020004", 0x80020004, "ab")
    write("half 0xcdef @ $80020006", 0x80020006, "cdef")
    write("byte[5] 1,2,3,4,5 @ $80020008", 0x80020008, "0102030405")
    write("RA_float 3 @ $80020010", 0x80020010, "21000003")
    statements += [".PO = $80022000", ".GR3 = $fedcba98"]
    write(".GR3 ->(32) PO+$00000000", 0x80022000, "fedcba98")
    write("CODE @ $80020020\n{\nli r3,77\nblr\n}", 0x80020020, "3860004d4e800020")
    hook = 0x90021000 if mem2 else 0x80021000
    statements += [f"CODE @ ${hook:08X}\n{{\nnop\nblr\n}}", f"HOOK @ ${hook:08X}\n{{\naddi r3,r3,5\n}}"]
    if args.console == "wii":
        write("word 0xdeadbeef @ $90020000", 0x90020000, "deadbeef")
        write("half 0x1234 @ $92020004", 0x92020004, "1234")
        write("CODE @ $90020020\n{\nli r3,99\nblr\n}", 0x90020020, "386000634e800020")
    source = work / ("handler-mem2.asm" if mem2 else "handler.asm")
    source.write_text("\n".join(statements)+"\n")
    subprocess.run([str(Path(args.assembler).resolve()), "--no-config", "--bug-fixes=true", "--set=extensions.branch_expressions=true", "--set=validation.console_only=true", "--set=extensions.expression_syntax=true", "--set=extensions.implicit_sections=true", "--set=extensions.additional_console_instructions=true", "--set=validation.reject_duplicate_labels=true", "--set=validation.strict_macro_calls=true", "--set=validation.reject_undefined_macros=true", "--set=validation.reject_address_annotations=true", "--set=validation.reject_data_overflow=true", "-i", "-q", str(source)], check=True, capture_output=True, timeout=15)
    gct = source.with_suffix(".GCT").read_bytes()
    handler = bytearray((Path(args.dolphin).resolve().parent / "Sys" / "codehandler.bin").read_bytes())
    # Match Dolphin's installer: patch the MMIO bank for Wii mode.
    if args.console == "wii":
        for i in range(0, len(handler),4):
            if struct.unpack_from(">I",handler,i)[0] == 0x3f00cc00:
                struct.pack_into(">I",handler,i,0x3f00cd00)
    handler[:4] = bytes.fromhex("d01f1bad")
    handler[7] = 1
    codeset = 0x80001800+len(handler)-8
    if mem2:
        # Relocate only the codeset pointer; execute the original handler in
        # MEM1. This keeps MEM2 hook and injected payload in branch range.
        locations = [i for i in range(0,len(handler)-4,4)
                     if struct.unpack_from(">I",handler,i)[0]==0x3de08000
                     and struct.unpack_from(">I",handler,i+4)[0]==0x61ef0000|(codeset&0xffff)]
        if len(locations)!=1:
            raise RuntimeError("cannot identify the handler codeset pointer")
        codeset=0x90001000
        struct.pack_into(">II",handler,locations[0],0x3de09000,0x61ef1000)
    image = handler[:-8] + (b"\0"*8 if mem2 else gct)
    if len(image)>0x1800:
        raise RuntimeError("handler/GCT exceeds Dolphin's reserved handler area")
    for offset in range(0,len(image),256):
        chunk = image[offset:offset+256]
        reply = remote.command(f"M{0x80001800+offset:x},{len(chunk):x}:{chunk.hex()}")
        if reply!="OK":
            raise RuntimeError(f"could not install Gecko handler at {offset}: {reply}")
    if mem2:
        for offset in range(0,len(gct),256):
            chunk=gct[offset:offset+256]
            if remote.command(f"M{codeset+offset:x},{len(chunk):x}:{chunk.hex()}")!="OK":
                raise RuntimeError("could not install MEM2 codeset")
    for register, value in [(1,0x80030000),(67,halt),(64,0x800018a8)]:
        if remote.command(f"P{register:x}={value:08x}")!="OK":
            raise RuntimeError("could not prepare handler call")
    remote.send("c")
    time.sleep(0.25)
    remote.sock.sendall(b"\x03")
    remote.read()
    pc = int(remote.command("p40"),16)
    if pc != halt:
        raise RuntimeError(f"Gecko handler did not return: PC={pc:08x}")
    rows=[]
    for name,address,want in expected:
        got = bytes.fromhex(remote.command(f"m{address:x},{len(want):x}"))
        rows.append(dict(check="Gecko: "+name, address=f"{address:08X}", expected=want.hex().upper(), actual=got.hex().upper(), passed=got==want))
    branch = int(remote.command(f"m{hook:x},4"),16)
    delta = branch & 0x03fffffc
    if delta & 0x02000000: delta -= 0x04000000
    target = (hook+delta)&0xffffffff
    valid_hook = branch>>26==18 and codeset<=target<codeset+len(gct)
    if valid_hook:
        injected = bytes.fromhex(remote.command(f"m{target:x},8"))
        tail=struct.unpack_from(">I",injected,4)[0]
        tail_delta=tail&0x03fffffc
        if tail_delta&0x02000000: tail_delta-=0x04000000
        valid_hook = injected[:4]==bytes.fromhex("38630005") and tail>>26==18 and (target+4+tail_delta)&0xffffffff==hook+4
    rows.append(dict(check="Gecko: C2 hook and return branch", actual=f"{branch:08X}", passed=valid_hook))
    if valid_hook:
        for register, value in [(3,37),(67,halt),(64,hook)]:
            if remote.command(f"P{register:x}={value:08x}")!="OK":
                raise RuntimeError("could not prepare C2 payload execution")
        remote.send("c")
        time.sleep(0.25)
        remote.sock.sendall(b"\x03")
        remote.read()
        pc = int(remote.command("p40"),16)
        result = int(remote.command("p3"),16)
        rows.append(dict(check="Gecko: C2 payload executes and returns", expected="0000002A", actual=f"{result:08X}", passed=result==42 and pc==halt))
    if mem2:
        for row in rows: row["check"]="MEM2 codeset: "+row["check"]
    return rows


def run(args):
    work = Path(args.work_dir).resolve()
    work.mkdir(parents=True, exist_ok=True)
    source = work / "runtime.asm"
    code, expected = program()
    source.write_text("Runtime validation\nCODE @ $80004000\n{\n"+"\n".join(code)+"\n}\n")
    subprocess.run([str(Path(args.assembler).resolve()), "--no-config", "--bug-fixes=true", "--set=extensions.branch_expressions=true", "--set=validation.console_only=true", "--set=extensions.expression_syntax=true", "--set=extensions.implicit_sections=true", "--set=extensions.additional_console_instructions=true", "--set=validation.reject_duplicate_labels=true", "--set=validation.strict_macro_calls=true", "--set=validation.reject_undefined_macros=true", "--set=validation.reject_address_annotations=true", "--set=validation.reject_data_overflow=true", "-i", "-q", str(source)], check=True, capture_output=True, timeout=15)
    gct = source.with_suffix(".GCT").read_bytes()
    size = struct.unpack_from(">I", gct, 12)[0]
    body = gct[16:16+size]
    text_section = body
    if args.console == "wii":
        # Dolphin's DolReader detects Wii DOLs by an mfspr HID4 instruction.
        # Place that marker after the halt loop; it is never executed.
        text_section += struct.pack(">I", 0x7c13fba6)
    text_section += b"\0" * (-len(text_section) % 32)
    header = bytearray(256)
    for offset, value in [(0,256), (0x48,0x80004000), (0x90,len(text_section)), (0xe0,0x80004000)]:
        struct.pack_into(">I", header, offset, value)
    dol = work / "runtime.dol"
    dol.write_bytes(header+text_section)
    profile = work / "user"
    (profile / "Config").mkdir(parents=True, exist_ok=True)
    with socket.socket() as available:
        available.bind(("127.0.0.1",0))
        port = available.getsockname()[1]
    (profile / "Config" / "Dolphin.ini").write_text(f"[Analytics]\nPermissionAsked = True\nEnabled = False\n[Interface]\nConfirmStop = False\n[Core]\nCPUThread = False\nCPUCore = 0\nGFXBackend = Null\n[General]\nGDBPort = {port}\n")
    cpu = 0 if args.cpu == "interpreter" else 1
    command = [str(Path(args.dolphin).resolve()), "-b", "-v", "Null", "-u", str(profile), "-e", str(dol), "-C", "Dolphin.Display.RenderToMain=True", "-C", "Dolphin.Interface.ConfirmStop=False", "-C", f"Dolphin.Core.CPUCore={cpu}"]
    startup = None
    if hasattr(subprocess, "STARTUPINFO"):
        startup = subprocess.STARTUPINFO()
        startup.dwFlags |= subprocess.STARTF_USESHOWWINDOW
        startup.wShowWindow = 0
    with (work / "dolphin-process.log").open("w") as log:
        child = subprocess.Popen(command, stdout=log, stderr=log, startupinfo=startup)
        try:
            deadline = time.monotonic()+20
            while True:
                if child.poll() is not None:
                    raise RuntimeError(f"Dolphin exited with {child.returncode}; see {work/'dolphin-process.log'}")
                try:
                    sock = socket.create_connection(("127.0.0.1",port), timeout=0.2)
                    break
                except OSError:
                    if time.monotonic()>deadline:
                        raise RuntimeError("Dolphin debugger did not become available within 20 seconds")
                    time.sleep(0.1)
            with sock:
                remote = Remote(sock)
                print("Debugger connected", flush=True)
                print("Initial stop:", remote.command("?"), flush=True)
                msr = int(remote.command("p41"),16)
                print(f"Initial MSR: {msr:08x}", flush=True)
                pvr = remote.command("p57")
                if remote.command(f"P41={msr|0x2000:08x}")!="OK":
                    raise RuntimeError("could not enable floating point")
                halt = 0x80004000+len(body)-4
                if remote.command(f"Z0,{halt:x},4")!="OK":
                    raise RuntimeError("could not set completion breakpoint")
                remote.send("c")
                time.sleep(0.25)
                # This Dolphin build does not notify the client immediately
                # on every breakpoint. An explicit break requests its status.
                sock.sendall(b"\x03")
                stop = remote.read()
                pc = int(remote.command("p40"),16)
                if pc!=halt:
                    raise RuntimeError(f"unexpected stop {stop} at {pc:08x}; expected {halt:08x}")
                data = bytes.fromhex(remote.command(f"m80010000,{len(expected)*4:x}"))
                rows = []
                for i,(name,want) in enumerate(expected):
                    got = struct.unpack_from(">I", data, i*4)[0]
                    rows.append(dict(check=name, expected=f"{want:08X}", actual=f"{got:08X}", passed=got==want))
                rows.extend(validate_handler(remote,args,work,halt))
                if args.console=="wii":
                    rows.extend(validate_handler(remote,args,work,halt,mem2=True))
                report = dict(bug_fixes=True, branch_expressions=True, allow_non_console_instructions=False, emulator_sha256=hashlib.sha256(Path(args.dolphin).read_bytes()).hexdigest(), assembler_sha256=hashlib.sha256(Path(args.assembler).read_bytes()).hexdigest(), cpu=args.cpu, console=args.console, processor_version=pvr, format="standalone DOL plus Gecko handler", checks=rows)
                (work / "results.json").write_text(json.dumps(report,indent=2)+"\n")
                failed = [r for r in rows if not r["passed"]]
                print(f"{len(rows)-len(failed)}/{len(rows)} execution checks passed", flush=True)
                if failed:
                    raise RuntimeError(json.dumps(failed,indent=2))
        finally:
            if child.poll() is None:
                child.terminate()
                try:
                    child.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    child.kill()
                    child.wait(timeout=5)


if __name__ == "__main__":
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dolphin", required=True)
    parser.add_argument("--assembler", required=True)
    parser.add_argument("--work-dir", required=True)
    parser.add_argument("--console", choices=["gamecube","wii"], default="gamecube")
    parser.add_argument("--cpu", choices=["interpreter","jit"], default="interpreter")
    run(parser.parse_args())
