// A tiny LLVM pass plugin for the tests of `gocpp build -cxxflags=-fpass-plugin=...` and of the
// IR that `-emit-llvm` writes (cmd/gocpp/llvmplugin_test.go). It counts the functions that a module
// defines (declarations do not count) and appends "<source file>\t<count>" to the file named by
// $GOCPP_PLUGIN_COUNT_FILE (to stderr when it is not set). It runs
//
//   - at the end of the optimization pipeline of every clang compile that loads it
//     (-fpass-plugin=count.so; this is also true at -O0), and
//   - as the pass gocpp-count-functions of `opt -load-pass-plugin=count.so
//     -passes=gocpp-count-functions`.
//
// Built by the test with clang++ -shared -fPIC -fno-rtti against the headers of the LLVM that
// goes with clang (LLVM is built without RTTI, a plugin must be too).
#include "llvm/IR/Module.h"
#include "llvm/IR/PassManager.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Passes/PassPlugin.h"
#include "llvm/Support/raw_ostream.h"

#include <cstdio>
#include <cstdlib>
#include <string>

using namespace llvm;

namespace {

struct CountFunctions : PassInfoMixin<CountFunctions> {
    PreservedAnalyses run(Module& M, ModuleAnalysisManager&) {
        unsigned n = 0;
        for (Function& F : M) {
            if (!F.isDeclaration()) {
                n++;
            }
        }
        std::string line = M.getSourceFileName() + "\t" + std::to_string(n) + "\n";
        if (const char* path = std::getenv("GOCPP_PLUGIN_COUNT_FILE")) {
            // One write of one short line: compiles that run at the same time do not mix lines.
            if (FILE* f = std::fopen(path, "a")) {
                std::fwrite(line.data(), 1, line.size(), f);
                std::fclose(f);
            }
        } else {
            errs() << line;
        }
        return PreservedAnalyses::all();
    }
    // Runs also in a pipeline that skips optional passes (-O0, optnone).
    static bool isRequired() { return true; }
};

}  // namespace

extern "C" LLVM_ATTRIBUTE_WEAK PassPluginLibraryInfo llvmGetPassPluginInfo() {
    return {LLVM_PLUGIN_API_VERSION, "gocpp-count-functions", "1", [](PassBuilder& PB) {
                PB.registerPipelineParsingCallback(
                    [](StringRef Name, ModulePassManager& MPM, ArrayRef<PassBuilder::PipelineElement>) {
                        if (Name == "gocpp-count-functions") {
                            MPM.addPass(CountFunctions());
                            return true;
                        }
                        return false;
                    });
                PB.registerOptimizerLastEPCallback(
                    [](ModulePassManager& MPM, OptimizationLevel, ThinOrFullLTOPhase) {
                        MPM.addPass(CountFunctions());
                    });
            }};
}
