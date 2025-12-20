package codegen

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

// Config 代码生成配置
type Config struct {
	// 源接口或结构体的包路径
	PackagePath string
	// 源接口或结构体名称
	TypeName string
	// 生成的包装器名称
	WrapperName string
	// 客户端池字段名
	PoolFieldName string
	// 客户端类型（用于泛型）
	ClientType string
	// 输出文件路径
	OutputPath string
	// 是否包含 Prometheus 监控
	EnablePrometheus bool
	// 自定义方法名转换函数（可选）
	MethodNameTransform func(string) string
}

// MethodInfo 方法信息
type MethodInfo struct {
	Name            string
	ReceiverName    string
	Params          []ParamInfo
	Results         []ParamInfo
	HasContext      bool
	ContextParamIdx int
	HasError        bool
	ErrorResultIdx  int
}

// ParamInfo 参数信息
type ParamInfo struct {
	Name string
	Type string
}

// Generator 代码生成器
type Generator struct {
	config  Config
	methods []MethodInfo
	imports map[string]bool
}

// NewGenerator 创建新的代码生成器
func NewGenerator(config Config) *Generator {
	return &Generator{
		config:  config,
		imports: make(map[string]bool),
	}
}

// Generate 生成包装代码
func (g *Generator) Generate() error {
	// 1. 解析源类型
	if err := g.parseType(); err != nil {
		return fmt.Errorf("failed to parse type: %w", err)
	}

	// 2. 生成代码
	if err := g.generateCode(); err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	return nil
}

// parseType 解析类型并提取方法
func (g *Generator) parseType() error {
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedImports,
	}

	pkgs, err := packages.Load(cfg, g.config.PackagePath)
	if err != nil {
		return fmt.Errorf("failed to load package: %w", err)
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("no packages found")
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return fmt.Errorf("package has errors: %v", pkg.Errors)
	}

	// 查找类型
	obj := pkg.Types.Scope().Lookup(g.config.TypeName)
	if obj == nil {
		return fmt.Errorf("type %s not found in package %s", g.config.TypeName, g.config.PackagePath)
	}

	// 获取类型的方法集
	var methodSet *types.MethodSet
	switch t := obj.Type().(type) {
	case *types.Named:
		// 对于命名类型，获取指针类型的方法集（包含值接收者和指针接收者的方法）
		methodSet = types.NewMethodSet(types.NewPointer(t))
	case *types.Interface:
		// 对于接口，直接获取方法集
		methodSet = types.NewMethodSet(t)
	default:
		return fmt.Errorf("unsupported type: %T", t)
	}

	// 提取所有公有方法
	for i := 0; i < methodSet.Len(); i++ {
		sel := methodSet.At(i)
		method := sel.Obj().(*types.Func)

		// 只处理公有方法
		if !method.Exported() {
			continue
		}

		methodInfo, err := g.parseMethod(method, pkg)
		if err != nil {
			return fmt.Errorf("failed to parse method %s: %w", method.Name(), err)
		}

		g.methods = append(g.methods, methodInfo)
	}

	return nil
}

// parseMethod 解析方法签名
func (g *Generator) parseMethod(method *types.Func, pkg *packages.Package) (MethodInfo, error) {
	sig := method.Type().(*types.Signature)

	info := MethodInfo{
		Name:            method.Name(),
		ReceiverName:    "m",
		ContextParamIdx: -1,
		ErrorResultIdx:  -1,
	}

	// 解析参数
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		paramType := g.typeString(param.Type(), pkg)

		paramName := param.Name()
		if paramName == "" {
			paramName = fmt.Sprintf("arg%d", i)
		}

		info.Params = append(info.Params, ParamInfo{
			Name: paramName,
			Type: paramType,
		})

		// 检查是否有 context.Context 参数
		if paramType == "context.Context" {
			info.HasContext = true
			info.ContextParamIdx = i
		}

		// 收集导入
		g.collectImports(param.Type(), pkg)
	}

	// 解析返回值
	results := sig.Results()
	for i := 0; i < results.Len(); i++ {
		result := results.At(i)
		resultType := g.typeString(result.Type(), pkg)

		resultName := result.Name()
		if resultName == "" {
			resultName = fmt.Sprintf("ret%d", i)
		}

		info.Results = append(info.Results, ParamInfo{
			Name: resultName,
			Type: resultType,
		})

		// 检查是否有 error 返回值
		if resultType == "error" {
			info.HasError = true
			info.ErrorResultIdx = i
		}

		// 收集导入
		g.collectImports(result.Type(), pkg)
	}

	return info, nil
}

// typeString 将类型转换为字符串
func (g *Generator) typeString(t types.Type, pkg *packages.Package) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == pkg.Types {
			return "" // 同一个包，不需要包名
		}
		return p.Name()
	})
}

// collectImports 收集需要导入的包
func (g *Generator) collectImports(t types.Type, pkg *packages.Package) {
	switch typ := t.(type) {
	case *types.Named:
		if obj := typ.Obj(); obj != nil && obj.Pkg() != nil && obj.Pkg() != pkg.Types {
			g.imports[obj.Pkg().Path()] = true
		}
	case *types.Pointer:
		g.collectImports(typ.Elem(), pkg)
	case *types.Slice:
		g.collectImports(typ.Elem(), pkg)
	case *types.Array:
		g.collectImports(typ.Elem(), pkg)
	case *types.Map:
		g.collectImports(typ.Key(), pkg)
		g.collectImports(typ.Elem(), pkg)
	}
}

// generateCode 生成包装代码
func (g *Generator) generateCode() error {
	// 确保输出目录存在
	outputDir := filepath.Dir(g.config.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 创建输出文件
	f, err := os.Create(g.config.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	// 准备模板数据
	data := struct {
		PackageName      string
		Imports          []string
		WrapperName      string
		PoolFieldName    string
		ClientType       string
		Methods          []MethodInfo
		EnablePrometheus bool
	}{
		PackageName:      filepath.Base(filepath.Dir(g.config.OutputPath)),
		Imports:          g.getImportList(),
		WrapperName:      g.config.WrapperName,
		PoolFieldName:    g.config.PoolFieldName,
		ClientType:       g.config.ClientType,
		Methods:          g.methods,
		EnablePrometheus: g.config.EnablePrometheus,
	}

	// 执行模板
	tmpl := template.Must(template.New("wrapper").Funcs(template.FuncMap{
		"join":               strings.Join,
		"lower":              strings.ToLower,
		"toSnakeCase":        toSnakeCase,
		"paramList":          g.paramList,
		"paramNames":         g.paramNames,
		"resultList":         g.resultList,
		"resultNames":        g.resultNames,
		"hasMultipleReturns": func(m MethodInfo) bool { return len(m.Results) > 1 },
	}).Parse(wrapperTemplate))

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// getImportList 获取导入列表
func (g *Generator) getImportList() []string {
	imports := []string{"context"}

	if g.config.EnablePrometheus {
		imports = append(imports, "github.com/bighu630/clientPool/middleware")
	}

	// 添加从类型分析中收集的导入
	for imp := range g.imports {
		if imp != "" && imp != "context" {
			imports = append(imports, imp)
		}
	}

	return imports
}

// paramList 生成参数列表
func (g *Generator) paramList(params []ParamInfo) string {
	var parts []string
	for _, p := range params {
		parts = append(parts, fmt.Sprintf("%s %s", p.Name, p.Type))
	}
	return strings.Join(parts, ", ")
}

// paramNames 生成参数名列表
func (g *Generator) paramNames(params []ParamInfo) string {
	var parts []string
	for _, p := range params {
		parts = append(parts, p.Name)
	}
	return strings.Join(parts, ", ")
}

// resultList 生成返回值列表（带名称）
func (g *Generator) resultList(results []ParamInfo) string {
	if len(results) == 0 {
		return ""
	}
	var parts []string
	for _, r := range results {
		parts = append(parts, fmt.Sprintf("%s %s", r.Name, r.Type))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// resultNames 生成返回值名称列表
func (g *Generator) resultNames(results []ParamInfo) string {
	var parts []string
	for _, r := range results {
		parts = append(parts, r.Name)
	}
	return strings.Join(parts, ", ")
}

// toSnakeCase 转换为蛇形命名
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

const wrapperTemplate = `// Code generated by clientPool codegen. DO NOT EDIT.

package {{.PackageName}}

import (
{{range .Imports}}	"{{.}}"
{{end}}
)

{{range .Methods}}
// {{.Name}} wraps the client method with pool management and monitoring
func ({{.ReceiverName}} *{{$.WrapperName}}) {{.Name}}({{paramList .Params}}){{if .Results}} {{resultList .Results}}{{end}} {
{{if $.EnablePrometheus}}	{{if .HasContext}}ctx{{else}}_ctx{{end}} = context.WithValue({{if .HasContext}}ctx{{else}}context.Background(){{end}}, middleware.PrometheusMethodKey{}, "{{toSnakeCase .Name}}")
{{end}}	{{if .HasError}}{{if .Results}}{{resultNames .Results}} = {{end}}{{$.PoolFieldName}}.Do({{if .HasContext}}ctx{{else}}context.Background(){{end}}, func(ctx context.Context, client {{$.ClientType}}) error {
		{{if gt (len .Results) 1}}{{range $idx, $r := .Results}}{{if ne $idx $.ErrorResultIdx}}{{$r.Name}}, {{end}}{{end}}{{end}}err{{if eq (len .Results) 1}} :{{end}}= client.{{.Name}}({{paramNames .Params}})
		return err
	}){{else}}{{$.PoolFieldName}}.Do({{if .HasContext}}ctx{{else}}context.Background(){{end}}, func(ctx context.Context, client {{$.ClientType}}) error {
		{{if .Results}}{{resultNames .Results}} = {{end}}client.{{.Name}}({{paramNames .Params}})
		return nil
	}){{end}}
	return
}
{{end}}
`
