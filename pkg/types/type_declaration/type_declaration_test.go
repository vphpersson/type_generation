package type_declaration

import (
	"testing"
)

func TestQualifiedName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		declaration TypeDeclaration
		expected    string
	}{
		{
			name:        "interface declaration",
			declaration: &InterfaceDeclaration{Identifier: "Foo"},
			expected:    "Foo",
		},
		{
			name:        "type alias declaration",
			declaration: &TypeAliasDeclaration{Identifier: "Bar"},
			expected:    "Bar",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := testCase.declaration.QualifiedName(); got != testCase.expected {
				t.Errorf("QualifiedName() = %q, want %q", got, testCase.expected)
			}
		})
	}
}
