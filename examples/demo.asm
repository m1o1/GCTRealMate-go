Example Patch
.alias Target = 0x80001234
.macro LoadAddress(<reg>, <value>)
{
    .alias high = <value> >> 16
    .alias low = <value> & 0xffff
    lis <reg>, high
    ori <reg>, <reg>, low
}

HOOK @ $80001000
{
    %LoadAddress(r3, Target)
    cmpwi r3, 0
    beq %END%
    li r4, 1
}

op nop @ $80002000
string "Hello from Go" @ $80003000
.RESET
