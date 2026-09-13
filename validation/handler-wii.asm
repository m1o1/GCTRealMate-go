Gecko handler validation
word 0x89abcdef @ $80020000
byte 0xab @ $80020004
half 0xcdef @ $80020006
byte[5] 1,2,3,4,5 @ $80020008
RA_float 3 @ $80020010
.PO = $80022000
.GR3 = $fedcba98
.GR3 ->(32) PO+$00000000
CODE @ $80020020
{
li r3,77
blr
}
CODE @ $80021000
{
nop
blr
}
HOOK @ $80021000
{
addi r3,r3,5
}
word 0xdeadbeef @ $90020000
half 0x1234 @ $92020004
CODE @ $90020020
{
li r3,99
blr
}
