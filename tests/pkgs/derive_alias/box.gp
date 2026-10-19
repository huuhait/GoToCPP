package main

// box.gp does not import the package of Things: the generators write the alias as it is declared.

#Derive(Clone, Equal)
type Box struct {
	A     Things
	T     Table
	Names []string
}
