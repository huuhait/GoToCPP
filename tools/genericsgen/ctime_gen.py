#!/usr/bin/env python3
"""ctime_gen.py KIND N K OUT.gp [hand]: (the programs of docs/guides/generics-compile-time.md; ctime.py builds and times them)
 write a generics-heavy program.
KIND f: N generic functions, each instantiated with K of 10 types (N*K instances); 'hand' writes the same with concrete functions.
KIND t: N generic types with 3 methods each, each instantiated with K types (N*K instances).
KIND n: a chain of N generic types, Box0[T] holds Box1[T]..., instantiated at nesting depths 1..K."""
import sys
kind,N,K,out=sys.argv[1],int(sys.argv[2]),int(sys.argv[3]),sys.argv[4]
hand=len(sys.argv)>5
TYPES=[('int','1'),('int8','2'),('int64','3'),('uint16','4'),('float32','1.5'),('float64','2.5'),('string','"s"'),('bool','true'),('pt','pt{1, 2}'),('[2]int8','[2]int8{1, 2}')]
def tn(t): return t[0].replace('[2]','arr2_').replace('.','_')
L=['package main','','import "fmt"','','type pt struct{ x, y int }','']
if kind=='f':
    for i in range(N):
        if not hand:
            L.append(f'''func F{i}[T any](xs []T, f func(T) T) (int, T) {{
	var last T
	m := map[int]T{{}}
	for i, x := range xs {{
		y := f(x)
		m[i] = y
		last = y
	}}
	if len(m) > {i % 7} {{
		last = f(last)
	}}
	return len(m), last
}}
''')
        else:
            for t,_ in TYPES[:K]:
                L.append(f'''func F{i}_{tn((t,))}(xs []{t}, f func({t}) {t}) (int, {t}) {{
	var last {t}
	m := map[int]{t}{{}}
	for i, x := range xs {{
		y := f(x)
		m[i] = y
		last = y
	}}
	if len(m) > {i % 7} {{
		last = f(last)
	}}
	return len(m), last
}}
''')
    L.append('func main() {\n\ttotal := 0')
    for i in range(N):
        for t,v in TYPES[:K]:
            fn=f'F{i}' if not hand else f'F{i}_{tn((t,))}'
            call=f'{fn}[{t}]' if not hand else fn
            L.append(f'\t{{\n\t\tn, _ := {call}([]{t}{{{v}, {v}}}, func(x {t}) {t} {{ return x }})\n\t\ttotal += n\n\t}}')
    L.append('\tfmt.Println(total)\n}')
elif kind=='t':
    for i in range(N):
        if not hand:
            L.append(f'''type G{i}[T any] struct {{
	items []T
	n     int
}}

func (g *G{i}[T]) Add(v T) {{ g.items = append(g.items, v); g.n++ }}
func (g G{i}[T]) Len() int {{ return len(g.items) + {i % 5} }}
func (g G{i}[T]) First() T {{ return g.items[0] }}
''')
        else:
            for t,_ in TYPES[:K]:
                L.append(f'''type G{i}_{tn((t,))} struct {{
	items []{t}
	n     int
}}

func (g *G{i}_{tn((t,))}) Add(v {t}) {{ g.items = append(g.items, v); g.n++ }}
func (g G{i}_{tn((t,))}) Len() int {{ return len(g.items) + {i % 5} }}
func (g G{i}_{tn((t,))}) First() {t} {{ return g.items[0] }}
''')
    L.append('func main() {\n\ttotal := 0')
    for i in range(N):
        for t,v in TYPES[:K]:
            ty=f'G{i}[{t}]' if not hand else f'G{i}_{tn((t,))}'
            L.append(f'\t{{\n\t\tvar g {ty}\n\t\tg.Add({v})\n\t\ttotal += g.Len()\n\t\t_ = g.First()\n\t\tfmt.Sprintf("%T", g)\n\t}}')
    L.append('\tfmt.Println(total)\n}')
elif kind=='n':
    # Box0[T] struct{ v T; next *Box1[T]... }: N distinct generic types, Box(i)[T] embeds Box(i+1)[T]
    for i in range(N):
        inner = f'\tin Box{i+1}[T]\n' if i+1<N else ''
        L.append(f'''type Box{i}[T any] struct {{
	v T
{inner}}}

func (b *Box{i}[T]) Set(v T) {{ b.v = v }}
func (b Box{i}[T]) Get() T {{ return b.v }}
func (b Box{i}[T]) String() string {{ return fmt.Sprint("box", b.v) }}
''')
    L.append('type W[T any] struct{ v T }\nfunc (w W[T]) Get() T { return w.v }\n')
    L.append('func main() {\n\ttotal := 0')
    for d in range(1,K+1):
        ty='int'
        for _ in range(d): ty=f'W[{ty}]'
        L.append(f'\t{{\n\t\tvar b Box0[{ty}]\n\t\ttotal += len(b.String()) + len(fmt.Sprintf("%T", b))\n\t}}')
    for i in range(N):
        for t,v in TYPES[:4]:
            L.append(f'\t{{\n\t\tvar b Box{i}[{t}]\n\t\tb.Set({v})\n\t\ttotal += len(b.String())\n\t}}')
    L.append('\tfmt.Println(total)\n}')
open(out,'w').write('\n'.join(L)+'\n')
