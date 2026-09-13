Raw Gecko
* 04001234 60000000
* E0000000 80008000

Single Writes
op bge- 0x14C @ $80055444
byte 0x12 @ $80001100
half 0x3456 @ $80001102
word 0x789abcde @ $81001104
float 1.25 @ $80001108
scalar 2.5 @ $8000110c
address $80001234 @ $80001110
byte[3] 1,2,3 @ $80001200
half[2] 0x1234,0x5678 @ $80001210
word[2] 0x12345678,0xabcdef01 @ $80001220
double 1.5 @ $80001230
string "hello" @ $80001240

Assembly Blocks
HOOK @ $80044A34
{
    lwz r4, 0x8(r3)
    cmpwi r8, 0x1
    bne- %END%
    lis r12, 0x8059
    stw r15, -0x7D08(r12)
}
CODE @ $80001198
{
    addi r6, r7, 0x4C
    mr r3, r7
    addi r4, r7, 0x34
    addi r5, r7, 0x38
}
PULSE
{
    lis r3, 0x8020
    ori r3, r3, 0x0984
    icbi r0, r3
    isync
    blr
}

Macros and Aliases
.alias Address = 0x80546120
.macro LoadAddress(<reg>, <addr>)
{
    .alias hi = <addr> / 0x10000
    .alias lo = <addr> & 0xFFFF
    lis <reg>, hi
    ori <reg>, <reg>, lo
}
HOOK @ $80001234
{
    %LoadAddress(r3, Address)
loop:
    addi r3, r3, 1
    bne loop
    b %END%
}

Memory Two
op li r3,1 @ $90001000
word[2] 0x11223344,0x55667788 @ $90001010
CODE @ $90001020
{
    li r3,1
    blr
}

Gecko Directives
.BA = $80000000
.PO = $90000000
.GR3 = $12345678
.GR4 ^= 00000002
.GOTO->after
* 04001234 60000000
after:
.PO <- data
.ENDIF
.ENDIF_RESET
.RESET
data:
* 12345678 9ABCDEF0

Branch Targets
.alias Dest = 0x80005000
CODE @ $80004000
{
    bl $Dest
    b $80005004
}
