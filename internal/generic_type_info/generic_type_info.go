package generic_type_info

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	goTypes "go/types"
	"os"
	"reflect"
	"strings"

	"github.com/vphpersson/type_generation/pkg/types/generic_type_info"
	"github.com/vphpersson/type_generation/pkg/types/shape"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	motmedelReflect "github.com/Motmedel/utils_go/pkg/reflect"
)

var (
	ErrNilPackage      = errors.New("nil package")
	ErrNilScope        = errors.New("nil scope")
	ErrNotTypeName     = errors.New("not a type name")
	ErrNotNamed        = errors.New("not a named")
	ErrEmptyTypeParams = errors.New("empty type parameters")
	ErrNotStruct       = errors.New("not a struct")
	ErrEmptyTypeName   = errors.New("empty type name")
	ErrNotGeneric      = errors.New("not a generic type")
)

func detectShapeTypes(
	t goTypes.Type,
	paramSet map[*goTypes.TypeParam]struct{},
) (*goTypes.TypeParam, shape.Kind, bool) {
	switch tt := t.(type) {
	case *goTypes.TypeParam:
		if _, ok := paramSet[tt]; ok {
			return tt, shape.KindDirect, true
		}
		return nil, 0, false
	case *goTypes.Pointer:
		if p, _, ok := detectShapeTypes(tt.Elem(), paramSet); ok {
			return p, shape.KindPointer, true
		}
	case *goTypes.Slice:
		if p, _, ok := detectShapeTypes(tt.Elem(), paramSet); ok {
			return p, shape.KindSlice, true
		}
	case *goTypes.Array:
		if p, _, ok := detectShapeTypes(tt.Elem(), paramSet); ok {
			return p, shape.KindArray, true
		}
	case *goTypes.Map:
		if p, _, ok := detectShapeTypes(tt.Elem(), paramSet); ok {
			return p, shape.KindMapValue, true
		}
		if p, _, ok := detectShapeTypes(tt.Key(), paramSet); ok {
			return p, shape.KindMapKey, true
		}
	}

	return nil, 0, false
}

func discoverUsingTypesImporter(pkgPath string, typeName string) (*generic_type_info.GenericTypeInfo, error) {
	pkg, err := importer.Default().Import(pkgPath)
	if err != nil {
		return nil, motmedelErrors.NewWithTrace(fmt.Errorf("go importer default import: %w", err))
	}
	if pkg == nil {
		return nil, motmedelErrors.NewWithTrace(ErrNilPackage)
	}

	pkgScope := pkg.Scope()
	if pkgScope == nil {
		return nil, motmedelErrors.NewWithTrace(ErrNilScope)
	}

	object := pkgScope.Lookup(typeName)
	if object == nil {
		return nil, nil
	}

	objectWithName, ok := object.(*goTypes.TypeName)
	if !ok {
		return nil, motmedelErrors.NewWithTrace(ErrNotTypeName)
	}

	namedType, ok := objectWithName.Type().(*goTypes.Named)
	if !ok {
		return nil, motmedelErrors.NewWithTrace(ErrNotNamed)
	}

	structType, ok := namedType.Underlying().(*goTypes.Struct)
	if !ok {
		return nil, motmedelErrors.NewWithTrace(ErrNotStruct)
	}

	typeParameters := namedType.TypeParams()
	if typeParameters.Len() == 0 {
		return nil, motmedelErrors.NewWithTrace(ErrEmptyTypeParams)
	}

	parameterNamesSet := map[*goTypes.TypeParam]struct{}{}
	parameterNames := make([]string, typeParameters.Len())
	for i := range typeParameters.Len() {
		typeParameter := typeParameters.At(i)
		parameterNamesSet[typeParameter] = struct{}{}
		parameterNames[i] = typeParameter.Obj().Name()
	}

	fieldNameToShape := map[string]shape.Shape{}
	paramToField := map[string]string{}
	for i := range structType.NumFields() {
		field := structType.Field(i)
		typeParameter, kind, ok := detectShapeTypes(field.Type(), parameterNamesSet)
		if !ok {
			continue
		}

		name := field.Name()
		fieldShape := shape.Shape{Param: typeParameter.Obj().Name(), Kind: kind}
		fieldNameToShape[name] = fieldShape
		if _, exists := paramToField[fieldShape.Param]; !exists {
			paramToField[fieldShape.Param] = name
		}
	}

	return &generic_type_info.GenericTypeInfo{
		TypeParameterNames:           parameterNames,
		FieldNameToShape:             fieldNameToShape,
		TypeParameterNameToFieldName: paramToField,
	}, nil
}

func detectShapeAst(e ast.Expr, paramSet map[string]struct{}) (string, shape.Kind, bool) {
	switch ee := e.(type) {
	case *ast.Ident:
		if _, ok := paramSet[ee.Name]; ok {
			return ee.Name, shape.KindDirect, true
		}
	case *ast.StarExpr:
		if p, _, ok := detectShapeAst(ee.X, paramSet); ok {
			return p, shape.KindPointer, true
		}
	case *ast.ArrayType:
		if p, _, ok := detectShapeAst(ee.Elt, paramSet); ok {
			if ee.Len == nil {
				return p, shape.KindSlice, true
			}
			return p, shape.KindArray, true
		}
	case *ast.MapType:
		if p, _, ok := detectShapeAst(ee.Value, paramSet); ok {
			return p, shape.KindMapValue, true
		}
		if p, _, ok := detectShapeAst(ee.Key, paramSet); ok {
			return p, shape.KindMapKey, true
		}
	}

	return "", 0, false
}

func discoverInWorkingDir(typeName string) (*generic_type_info.GenericTypeInfo, error) {
	workingDirectoryPath, err := os.Getwd()
	if err != nil {
		return nil, motmedelErrors.NewWithTrace(fmt.Errorf("os getwd: %w", err))
	}

	packages, err := parser.ParseDir(token.NewFileSet(), workingDirectoryPath, nil, 0)
	if err != nil {
		return nil, motmedelErrors.NewWithTrace(
			fmt.Errorf("go parser parse dir: %w", err),
			workingDirectoryPath,
		)
	}

	for _, pkg := range packages {
		for _, file := range pkg.Files {
			for _, topLevelDeclaration := range file.Decls {
				genericDeclarationNode, ok := topLevelDeclaration.(*ast.GenDecl)
				if !ok || genericDeclarationNode.Tok != token.TYPE {
					continue
				}

				for _, spec := range genericDeclarationNode.Specs {
					// Find the type spec for the base type.

					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || typeSpec.Name == nil || typeSpec.Name.Name != typeName {
						continue
					}
					structType, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					// Extract the type parameters

					var paramNames []string
					paramSet := map[string]struct{}{}
					if typeParams := typeSpec.TypeParams; typeParams != nil {
						for _, field := range typeParams.List {
							for _, identifier := range field.Names {
								paramNames = append(paramNames, identifier.Name)
								paramSet[identifier.Name] = struct{}{}
							}
						}
					}
					// TODO: No parameters? Can this happen?
					if len(paramNames) == 0 {
						continue
					}

					fieldShapes := map[string]shape.Shape{}
					paramToField := map[string]string{}
					for _, field := range structType.Fields.List {
						if len(field.Names) == 0 {
							continue
						}

						// Check if the struct field's type uses any of the type parameters.
						param, kind, ok := detectShapeAst(field.Type, paramSet)
						if !ok {
							continue
						}

						for _, identifier := range field.Names {
							fieldShapes[identifier.Name] = shape.Shape{Param: param, Kind: kind}
							if _, exists := paramToField[param]; !exists {
								paramToField[param] = identifier.Name
							}
						}
					}

					return &generic_type_info.GenericTypeInfo{
						TypeParameterNames:           paramNames,
						FieldNameToShape:             fieldShapes,
						TypeParameterNameToFieldName: paramToField,
					}, nil
				}
			}
		}
	}

	return nil, nil
}

func fullTypeName(t reflect.Type) string {
	name := t.Name()
	if name == "" {
		return t.String()
	}
	if pkgPath := t.PkgPath(); pkgPath != "" {
		return pkgPath + "." + name
	}
	return name
}

func parseTypeArgs(name string) []string {
	idx := strings.Index(name, "[")
	if idx == -1 {
		return nil
	}
	inner := name[idx+1 : len(name)-1]

	var args []string
	depth := 0
	start := 0
	for i, c := range inner {
		switch c {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[start:i]))
				start = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(inner[start:]))
	return args
}

func matchTypeArg(fieldType reflect.Type, typeArgs []string) (int, shape.Kind, bool) {
	ftn := fullTypeName(fieldType)
	for i, arg := range typeArgs {
		if ftn == arg {
			return i, shape.KindDirect, true
		}
	}

	switch fieldType.Kind() {
	case reflect.Ptr:
		ftn = fullTypeName(fieldType.Elem())
		for i, arg := range typeArgs {
			if ftn == arg {
				return i, shape.KindPointer, true
			}
		}
	case reflect.Slice:
		ftn = fullTypeName(fieldType.Elem())
		for i, arg := range typeArgs {
			if ftn == arg {
				return i, shape.KindSlice, true
			}
		}
	case reflect.Array:
		ftn = fullTypeName(fieldType.Elem())
		for i, arg := range typeArgs {
			if ftn == arg {
				return i, shape.KindArray, true
			}
		}
	case reflect.Map:
		ftn = fullTypeName(fieldType.Elem())
		for i, arg := range typeArgs {
			if ftn == arg {
				return i, shape.KindMapValue, true
			}
		}
		ftn = fullTypeName(fieldType.Key())
		for i, arg := range typeArgs {
			if ftn == arg {
				return i, shape.KindMapKey, true
			}
		}
	}

	return 0, 0, false
}

func discoverUsingReflection(structType reflect.Type) (*generic_type_info.GenericTypeInfo, error) {
	typeArgs := parseTypeArgs(structType.Name())
	if len(typeArgs) == 0 {
		return nil, nil
	}

	paramNames := make([]string, len(typeArgs))
	for i := range typeArgs {
		paramNames[i] = fmt.Sprintf("T%d", i)
	}

	fieldNameToShape := map[string]shape.Shape{}
	paramToField := map[string]string{}

	for i := range structType.NumField() {
		field := structType.Field(i)
		argIdx, kind, ok := matchTypeArg(field.Type, typeArgs)
		if !ok {
			continue
		}

		paramName := paramNames[argIdx]
		fieldNameToShape[field.Name] = shape.Shape{Param: paramName, Kind: kind}
		if _, exists := paramToField[paramName]; !exists {
			paramToField[paramName] = field.Name
		}
	}

	return &generic_type_info.GenericTypeInfo{
		TypeParameterNames:           paramNames,
		FieldNameToShape:             fieldNameToShape,
		TypeParameterNameToFieldName: paramToField,
	}, nil
}

func GetGenericTypeInfo(structType reflect.Type) (*generic_type_info.GenericTypeInfo, error) {
	structType = motmedelReflect.RemoveIndirection(structType)
	if structType.Kind() != reflect.Struct {
		return nil, motmedelErrors.NewWithTrace(ErrNotStruct)
	}

	typeName, isGenericType := motmedelReflect.GetTypeName(structType)
	if typeName == "" {
		return nil, motmedelErrors.NewWithTrace(ErrEmptyTypeName)
	}
	if !isGenericType {
		return nil, motmedelErrors.NewWithTrace(ErrNotGeneric)
	}

	var genericTypeInfo *generic_type_info.GenericTypeInfo

	var workingDirErr error
	var importerErr error
	var reflectionErr error
	genericTypeInfo, workingDirErr = discoverInWorkingDir(typeName)

	if genericTypeInfo == nil {
		genericTypeInfo, importerErr = discoverUsingTypesImporter(structType.PkgPath(), typeName)
	}

	if genericTypeInfo == nil {
		genericTypeInfo, reflectionErr = discoverUsingReflection(structType)
	}

	if genericTypeInfo == nil {
		return nil, errors.Join(workingDirErr, importerErr, reflectionErr)
	}

	return genericTypeInfo, nil
}
