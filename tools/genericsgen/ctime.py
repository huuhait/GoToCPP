#!/usr/bin/env python3
"""ctime.py CONFIGS MODES TAG: build the generic programs of ctime_gen.py with gocpp (GOCPP=path) and print the CPU seconds of each build.

CONFIGS is JSON: [["f",20,10,false],["f",20,10,true],["t",20,10,false]] (kind, N, K, hand-written twin);
MODES is a list from O0,O2,release; the compilers are g++ and clang++. Needs the dev shell (GOCPP_CXX is set for each).
Example: GOCPP=$PWD/gocpp python3 tools/genericsgen/ctime.py '[["f",100,5,false],["f",100,5,true]]' O0 mine"""
import os, subprocess, sys, shutil, re, time, json
HERE=os.path.dirname(os.path.abspath(__file__))
WORK=os.environ.get('CTIME_WORK','/tmp/gocpp-ctime') # programs and the cache folder (the runtime is built there once)
GOCPP=os.environ.get('GOCPP','gocpp')
XDG=WORK+'/xdg'
os.makedirs(WORK,exist_ok=True)
REPEAT=int(os.environ.get('REPEAT','3'))
MODES={'O0':[], 'O2':['-cxxflags=-O2'], 'release':['-release']}
def build(dirp, cxx, mode, tag):
    env=dict(os.environ,XDG_CACHE_HOME=XDG,GOCPP_GATE_PHASE='09',GOCPP_CXX=cxx,GOCPP_MODCACHE='off',GOCPP_OFFLINE='1',GOCPP_NOOBJCACHE='1',GOCPP_NOUPTODATE='1')
    cmd=['/usr/bin/env','time','-f','%e %U %S %M',GOCPP,'build','-o',dirp+'/x.bin']+MODES[mode]+[dirp+'/main.gp']
    best=None
    for _ in range(REPEAT):
        r=subprocess.run(cmd,cwd=dirp,env=env,capture_output=True,text=True)
        if r.returncode: return None,r.stderr[-400:]
        last=r.stderr.strip().splitlines()[-1].split()
        wall,u,s_,kb=float(last[0]),float(last[1]),float(last[2]),int(last[3])
        m={'wall':wall,'cpu':round(u+s_,2),'rss_mb':kb//1024}
        if best is None or m['cpu']<best['cpu']: best=m
    return best,None
def prog(kind,N,K,hand):
    d=f'{WORK}/w_{kind}{N}x{K}{"h" if hand else ""}'
    os.makedirs(d,exist_ok=True)
    args=['python3',HERE+'/ctime_gen.py',kind,str(N),str(K),d+'/main.gp']+(['hand'] if hand else [])
    subprocess.run(args,check=True)
    return d
configs=json.loads(sys.argv[1])
res=[]
for cxx in ['g++','clang++']:
    for mode in sys.argv[2].split(','):
        hello=f'{WORK}/w_hello'; os.makedirs(hello,exist_ok=True)
        open(hello+'/main.gp','w').write('package main\n\nimport "fmt"\n\nfunc main() { fmt.Println("hi") }\n')
        build(hello,cxx,mode,'warm')
        h,_=build(hello,cxx,mode,'hello')
        print(f'{cxx} {mode} hello: {h}',flush=True)
        for kind,N,K,hand in configs:
            d=prog(kind,N,K,hand)
            m,err=build(d,cxx,mode,'p')
            print(f'{cxx} {mode} {kind} N={N} K={K}{" hand" if hand else ""}: {m} {err or ""}',flush=True)
            res.append((cxx,mode,kind,N,K,hand,m))
json.dump(res,open(WORK+'/results_'+sys.argv[3]+'.json','w'))
