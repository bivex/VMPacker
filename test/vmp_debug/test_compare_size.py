import re

c_code = open('stub/linux/x86_64/vm_decode.h').read()
go_code = open('pkg/vm/opcodes.go').read()
go_disasm = open('pkg/vm/disasm.go').read()

# C sizes
c_sizes = {}
current_cases = []
for line in c_code.split('\n'):
    line = line.strip()
    if line.startswith('case OP_ID_'):
        op = line.split('case OP_ID_')[1].split(':')[0]
        current_cases.append(op)
    elif line.startswith('return '):
        try:
            sz = int(line.split('return ')[1].replace(';', ''))
            for op in current_cases:
                c_sizes[op] = sz
            current_cases = []
        except:
            pass

# Go sizes from disasm.go
go_sizes = {}
for line in go_disasm.split('\n'):
    line = line.strip()
    if 'Op' in line and '{"' in line and '"' in line:
        try:
            parts = line.split('{')[1].split('}')[0].split(',')
            name = parts[0].strip().strip('"')
            sz = int(parts[1].strip())
            # Convert OpName to OP_ID_NAME
            op_var = line.split(':')[0].strip()
            if op_var.startswith('OpId'): continue
            if op_var.startswith('Op'):
                op_name = op_var[2:].upper()
                go_sizes[op_name] = sz
        except:
            pass

for op, gsz in go_sizes.items():
    if op in c_sizes:
        if c_sizes[op] != gsz:
            print(f"MISMATCH: {op} C={c_sizes[op]} Go={gsz}")
    else:
        print(f"MISSING IN C: {op} (Go={gsz})")
