// Package check will type-check gocpp packages with the stock go/types.
// FakeImportC makes C/C++ names "opaque": go/types accepts them without a
// type, and the C++ compiler checks them later. On top of that, the opaque
// pass records which expressions come from C/C++, and the unsupported-feature
// checker rejects every feature that is not built yet with a message naming
// the phase that brings it (a phase gate), so nobody gets broken C++.
//
// Phase 01 fills it (steps 1.10 to 1.12). Each later phase removes the gates
// of the features it builds.
package check
