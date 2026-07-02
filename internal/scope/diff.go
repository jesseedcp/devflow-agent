package scope

type DiffResult struct {
	InScope       []string
	OutOfScope    []string
	MissingTests  []string
	DeclaredFiles []string
}

func CompareChangedFiles(decl Declaration, changedFiles []string) DiffResult {
	declared := make(map[string]bool)
	testDeclared := make(map[string]bool)
	var result DiffResult
	for _, file := range decl.SourceFiles {
		declared[file] = true
		result.DeclaredFiles = append(result.DeclaredFiles, file)
	}
	for _, file := range decl.TestFiles {
		declared[file] = true
		testDeclared[file] = true
		result.DeclaredFiles = append(result.DeclaredFiles, file)
	}
	changedSet := make(map[string]bool)
	for _, changed := range changedFiles {
		changedSet[changed] = true
		if declared[changed] {
			result.InScope = append(result.InScope, changed)
		} else {
			result.OutOfScope = append(result.OutOfScope, changed)
		}
	}
	for file := range testDeclared {
		if !changedSet[file] {
			result.MissingTests = append(result.MissingTests, file)
		}
	}
	return result
}
