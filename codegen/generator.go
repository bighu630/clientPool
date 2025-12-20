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
		// 对于命名类型，检查底层是否为接口
		if _, ok := t.Underlying().(*types.Interface); ok {
			// 接口类型，直接使用类型本身
			methodSet = types.NewMethodSet(t)
		} else {
			// 结构体等类型，获取指针类型的方法集（包含值接收者和指针接收者的方法）
			methodSet = types.NewMethodSet(types.NewPointer(t))
		}
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
		SourcePackage    string
		Methods          []MethodInfo
		EnablePrometheus bool
	}{
		PackageName:      filepath.Base(filepath.Dir(g.config.OutputPath)),
		Imports:          g.getImportList(),
		WrapperName:      g.config.WrapperName,
		PoolFieldName:    g.config.PoolFieldName,
		ClientType:       g.config.ClientType,
		SourcePackage:    g.config.PackagePath,
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
		"nonErrorResults":    g.nonErrorResults,
		"getErrorResultName": g.getErrorResultName,
		"hasMultipleReturns": func(m MethodInfo) bool { return len(m.Results) > 1 },
	}).Parse(wrapperTemplate))

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// getImportList 获取导入列表
func (g *Generator) getImportList() []string {
	imports := []string{
		"context",
		"time",
		"github.com/bighu630/clientPool",
		"github.com/bighu630/clientPool/middleware",
	}

	// 添加源类型的包（如果不在当前包）
	if g.config.PackagePath != "" {
		outputPkg := filepath.Base(filepath.Dir(g.config.OutputPath))
		sourcePkg := filepath.Base(g.config.PackagePath)

		// 如果源包和输出包不同，需要导入源包
		if outputPkg != sourcePkg {
			imports = append(imports, g.config.PackagePath)
		}
	}

	// 添加从类型分析中收集的导入
	for imp := range g.imports {
		if imp != "" && imp != "context" &&
			imp != "github.com/bighu630/clientPool" &&
			imp != "github.com/bighu630/clientPool/middleware" &&
			imp != g.config.PackagePath {
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

// nonErrorResults 生成非错误的返回值名称列表
func (g *Generator) nonErrorResults(m MethodInfo) string {
	var parts []string
	for i, r := range m.Results {
		if i != m.ErrorResultIdx {
			parts = append(parts, r.Name)
		}
	}
	return strings.Join(parts, ", ")
}

// getErrorResultName 获取error返回值的名称
func (g *Generator) getErrorResultName(m MethodInfo) string {
	if m.HasError && m.ErrorResultIdx >= 0 && m.ErrorResultIdx < len(m.Results) {
		return m.Results[m.ErrorResultIdx].Name
	}
	return ""
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

// MultiSt wraps multiple clients with load balancing and middleware support
type {{.WrapperName}} struct {
	{{.PoolFieldName}} *clientPool.ClientPool[{{.ClientType}}]
}

// New{{.WrapperName}} creates a new {{.WrapperName}} instance
func New{{.WrapperName}}(maxFails int, cooldown time.Duration, balancer clientPool.BalancerType) *{{.WrapperName}} {
	return &{{.WrapperName}}{
		{{.PoolFieldName}}: clientPool.NewClientPool[{{.ClientType}}](maxFails, cooldown, balancer),
	}
}

// AddClient adds a client to the pool with a name and weight
func (m *{{.WrapperName}}) AddClient(client {{.ClientType}}, name string, weight int) {
	m.{{.PoolFieldName}}.AddClient(client, name, weight)
}

// RegisterMiddleware registers a middleware to the pool
func (m *{{.WrapperName}}) RegisterMiddleware(mw middleware.Middleware[{{.ClientType}}]) {
	m.{{.PoolFieldName}}.RegisterMiddleware(mw)
}

{{range .Methods}}
// {{.Name}} wraps the client method with pool management and monitoring
func ({{.ReceiverName}} *{{$.WrapperName}}) {{.Name}}({{paramList .Params}}){{if .Results}} {{resultList .Results}}{{end}} {
{{if $.EnablePrometheus}}	{{if .HasContext}}ctx = context.WithValue(ctx, middleware.PrometheusMethodKey{}, "{{toSnakeCase .Name}}"){{else}}ctx := context.WithValue(context.Background(), middleware.PrometheusMethodKey{}, "{{toSnakeCase .Name}}"){{end}}
{{end}}	{{if .HasError}}{{getErrorResultName .}} = {{.ReceiverName}}.{{$.PoolFieldName}}.Do({{if .HasContext}}ctx{{else}}ctx{{end}}, func(ctx context.Context, client {{$.ClientType}}) error {
		{{$nonErr := nonErrorResults .}}{{if $nonErr}}{{$nonErr}}, {{end}}{{getErrorResultName .}} = client.{{.Name}}({{paramNames .Params}})
		return {{getErrorResultName .}}
	}){{else}}{{.ReceiverName}}.{{$.PoolFieldName}}.Do({{if .HasContext}}ctx{{else}}ctx{{end}}, func(ctx context.Context, client {{$.ClientType}}) error {
		{{if .Results}}{{resultNames .Results}} = {{end}}client.{{.Name}}({{paramNames .Params}})
		return nil
	}){{end}}
	return
}
{{end}}
`
