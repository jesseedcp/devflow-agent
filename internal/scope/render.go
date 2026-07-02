package scope

import (
	"fmt"
	"strings"
)

func RenderDeclaration(title string, decl Declaration) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Change Scope: %s\n\n", title)
	writeSection(&builder, "Source Files", decl.SourceFiles)
	writeSection(&builder, "Test Files", decl.TestFiles)
	writeSection(&builder, "Out Of Scope", decl.OutOfScope)
	return builder.String()
}

func ParseDeclaration(text string) Declaration {
	var decl Declaration
	section := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch line {
		case "## Source Files":
			section = "source"
			continue
		case "## Test Files":
			section = "test"
			continue
		case "## Out Of Scope":
			section = "out"
			continue
		}
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		value = strings.Trim(value, "`")
		if value == "" {
			continue
		}
		switch section {
		case "source":
			decl.SourceFiles = append(decl.SourceFiles, value)
		case "test":
			decl.TestFiles = append(decl.TestFiles, value)
		case "out":
			decl.OutOfScope = append(decl.OutOfScope, value)
		}
	}
	return decl
}

func writeSection(builder *strings.Builder, title string, files []string) {
	fmt.Fprintf(builder, "## %s\n\n", title)
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		fmt.Fprintf(builder, "- `%s`\n", file)
	}
	builder.WriteString("\n")
}
