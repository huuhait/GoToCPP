#!/usr/bin/env python3
"""Replay captured real gocpp builds with isolated experimental header units.

Run with python3 -I, inside the development shell, after prepare.sh. This does not
alter the compiler, runtime cache, or captured source. See protocol.txt.
"""

import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def save(path, data):
    path.write_text(json.dumps(data, indent=2) + "\n")


def without_pairs(arguments, options):
    result = []
    index = 0
    while index < len(arguments):
        if arguments[index] in options:
            index += 2
        else:
            result.append(arguments[index])
            index += 1
    return result


def invoke(command, directory, stem, timed=False):
    # Each child has its own time file: concurrent compiles cannot mix CPU totals.
    actual = [shutil.which("timeout"), "--kill-after=5s", "180s"]
    if timed:
        actual += [shutil.which("time"), "-f", "%e %U %S %M", "-o", str(stem) + ".time"]
    actual += command
    started = time.monotonic()
    with open(str(stem) + ".stdout", "wb") as stdout:
        with open(str(stem) + ".stderr", "wb") as stderr:
            result = subprocess.run(actual, cwd=directory, stdout=stdout, stderr=stderr)
    record = {"argv": command, "cwd": str(directory), "exit": result.returncode}
    if timed:
        record["elapsed"] = time.monotonic() - started
        try:
            fields = Path(str(stem) + ".time").read_text().splitlines()[-1].split()
            record.update(wall=float(fields[0]), user=float(fields[1]),
                          system=float(fields[2]), rss_kb=int(fields[3]))
        except (OSError, IndexError, ValueError) as error:
            # A timeout can kill GNU time before it writes: retain the failure record.
            record["timing_error"] = str(error)
    save(Path(str(stem) + ".json"), record)
    return record


def captured(work, compiler):
    rows = [json.loads(line) for line in (work / compiler / "commands.jsonl").read_text().splitlines()]
    units = {}
    links = {}
    for row in rows:
        arguments = row["argv"]
        if "-c" in arguments:
            source = arguments[arguments.index("-c") + 1]
            if "/gen/" in source and source.endswith(".cpp"):
                units.setdefault(row["cwd"], []).append(row)
        elif any(argument.endswith("libgocpprt.a") for argument in arguments):
            links[row["cwd"]] = row
    workloads = {}
    for directory, commands in units.items():
        name = "t20" if len(commands) == 3 else "hello"
        assert len(commands) == (3 if name == "t20" else 2)
        assert directory in links
        workloads[name] = {"units": commands, "link": links[directory]}
    assert set(workloads) == {"hello", "t20"}
    return workloads


def setup(work, compiler, workload, mode, label, captured_build):
    directory = work / "replay" / compiler / workload / mode / label
    directory.mkdir(parents=True, exist_ok=False)
    original = Path(captured_build["link"]["cwd"])
    generated = directory / "gen"
    shutil.copytree(original / "gen", generated)
    first = captured_build["units"][0]["argv"]
    flags = without_pairs(first[1:first.index("-c")], {"-include", "-include-pch"})
    flags = [argument.replace(str(original / "gen"), str(generated)) for argument in flags]
    include = Path(flags[flags.index("-I") + 1])
    package = generated / "gpkg/command-line-arguments/pkg.hpp"
    bmi = directory / "header.pcm"
    extra = []
    if mode == "pch":
        header = directory / "rt.hpp"
        header.write_text("#include <gocpp/rt.hpp>\n")
        extension = ".gch" if compiler == "gcc" else ".pch"
        bmi = Path(str(header) + extension)
        pch_flags = without_pairs(flags, {"-iquote"})
        build = [first[0]] + pch_flags + ["-c", "-x", "c++-header", str(header), "-o", str(bmi)]
        extra = ["-include" if compiler == "gcc" else "-include-pch",
                 str(header) if compiler == "gcc" else str(bmi)]
    else:
        header = include / "gocpp/rt.hpp" if mode == "rt-hu" else package
        if compiler == "gcc":
            extra = ["-fmodules-ts"]
            build = [first[0]] + flags + extra + ["-x", "c++-header", str(header)]
        else:
            kind = "system" if mode == "rt-hu" else "user"
            build = [first[0]] + flags + ["-fmodule-header=" + kind, "-x", "c++-header",
                                         "--precompile", str(header), "-o", str(bmi)]
            extra = ["-fmodule-file=" + str(bmi)]
        for source in generated.rglob("*"):
            if source.suffix not in {".cpp", ".hpp"}:
                continue
            content = source.read_text()
            if mode == "rt-hu":
                content = content.replace("#include <gocpp/rt.hpp>", "import <gocpp/rt.hpp>;")
            elif source.suffix == ".cpp":
                content = content.replace('#include "gpkg/command-line-arguments/pkg.hpp"',
                                          'import "' + str(package) + '";')
            source.write_text(content)

    units = []
    replacements = {}
    for row in sorted(captured_build["units"], key=lambda item: item["argv"]):
        arguments = row["argv"]
        source = arguments[arguments.index("-c") + 1]
        old_object = arguments[arguments.index("-o") + 1]
        new_object = str(directory / Path(old_object).name)
        replacements[old_object] = new_object
        units.append([first[0]] + flags + extra + ["-c",
                     source.replace(str(original / "gen"), str(generated)), "-o", new_object])
    link = [replacements.get(argument, argument) for argument in captured_build["link"]["argv"]]
    link[link.index("-o") + 1] = str(directory / "program")
    frontend = directory / "frontend.cpp"
    if mode == "pkg-hu":
        frontend.write_text('import "' + str(package) + '";\n')
    else:
        frontend.write_text('#include "gpkg/command-line-arguments/pkg.hpp"\n')
    syntax = [first[0]] + flags + extra + ["-fsyntax-only", str(frontend)]
    manifest = {"construction": build, "units": units, "link": link, "frontend": syntax,
                "input_sha256": digest(work / "input" / workload / "main.gp"),
                "original_header_sha256": digest(original / "gen/gpkg/command-line-arguments/pkg.hpp"),
                "original_header_bytes": (original / "gen/gpkg/command-line-arguments/pkg.hpp").stat().st_size}
    save(directory / "manifest.json", manifest)
    return directory, manifest


def execute(directory, manifest, workload, timed):
    result = {"load_start": os.getloadavg()}
    construction = invoke(manifest["construction"], directory, directory / "construction", timed)
    result["construction"] = construction
    if construction["exit"]:
        result["status"] = "construction-failed"
        save(directory / "result.json", result)
        return result

    # Finish all program units, including descriptor/init, even if one fails.
    started = time.monotonic()
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        futures = [pool.submit(invoke, command, directory, directory / ("unit-" + str(index)), timed)
                   for index, command in enumerate(manifest["units"])]
        result["units"] = [future.result() for future in futures]
    if any(unit["exit"] for unit in result["units"]):
        result["status"] = "compile-failed"
    else:
        result["link"] = invoke(manifest["link"], directory, directory / "link", timed)
        result["reuse_wall"] = time.monotonic() - started
        result["status"] = "link-failed" if result["link"]["exit"] else "built"
        if result["status"] == "built":
            binary = directory / "program"
            run = invoke([str(binary)], directory, directory / "run")
            expected_stdout = b"600\n" if workload == "t20" else b""
            expected_stderr = b"" if workload == "t20" else b"hello\n"
            equal = (Path(str(directory / "run") + ".stdout").read_bytes() == expected_stdout
                     and Path(str(directory / "run") + ".stderr").read_bytes() == expected_stderr)
            result["status"] = "pass" if run["exit"] == 0 and equal else "output-mismatch"
            result["binary_sha256"] = digest(binary)
            result["frontend"] = invoke(manifest["frontend"], directory, directory / "frontend", timed)
    result["load_end"] = os.getloadavg()
    if timed and result["status"] == "pass":
        steps = result["units"] + [result["link"]]
        result["reuse_cpu"] = sum(step["user"] + step["system"] for step in steps)
    save(directory / "result.json", result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("work", type=Path)
    parser.add_argument("stage", choices=["probe", "measure"])
    args = parser.parse_args()
    work = args.work.resolve()
    summary = {}
    for compiler in ["gcc", "clang"]:
        builds = captured(work, compiler)
        for workload in ["hello", "t20"]:
            modes = ["pch", "rt-hu", "pkg-hu"]
            if args.stage == "measure":
                modes = [mode for mode in modes if json.loads(
                    (work / "replay" / compiler / workload / mode / "probe/result.json").read_text()
                )["status"] == "pass"]
            orders = [modes] if args.stage == "probe" else [modes, list(reversed(modes))]
            for index, order in enumerate(orders):
                label = "probe" if args.stage == "probe" else "round-" + str(index + 1)
                for mode in order:
                    directory, manifest = setup(work, compiler, workload, mode, label, builds[workload])
                    result = execute(directory, manifest, workload, args.stage == "measure")
                    key = "/".join([compiler, workload, mode, label])
                    summary[key] = result
                    print(key, result["status"], flush=True)
    save(work / (args.stage + "-summary.json"), summary)


if __name__ == "__main__":
    main()
