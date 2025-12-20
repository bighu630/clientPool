package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bighu630/clientPool/codegen"
)

func main() {
	var (
		packagePath      = flag.String("package", "", "源接口或结构体的包路径 (必需)")
		typeName         = flag.String("type", "", "源接口或结构体名称 (可选，从-client自动推断)")
		wrapperName      = flag.String("wrapper", "", "生成的包装器名称 (可选，自动生成)")
		poolFieldName    = flag.String("pool", "pool", "客户端池字段名")
		clientType       = flag.String("client", "", "客户端类型 (必需，如: *rpc.Client 或 codegen.It)")
		outputPath       = flag.String("output", "", "输出文件路径 (可选，自动生成)")
		enablePrometheus = flag.Bool("prometheus", true, "是否包含 Prometheus 监控")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "clientPool 代码生成工具\n\n")
		fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "选项:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n示例:\n")
		fmt.Fprintf(os.Stderr, "  简化版: %s -package=github.com/bighu630/clientPool/codegen -client='codegen.It'\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  完整版: %s -package=github.com/gagliardetto/solana-go/rpc -client='*rpc.Client' -wrapper=RPCPool -output=./generated/rpc_pool/client.go\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n")
	}

	flag.Parse()

	// 验证必需参数
	if *packagePath == "" || *clientType == "" {
		flag.Usage()
		os.Exit(1)
	}

	// 从 clientType 中提取类型信息
	extractedType := extractTypeName(*clientType)

	// 如果没有指定 typeName，使用提取的类型
	if *typeName == "" {
		*typeName = extractedType
	}

	// 如果没有指定 wrapperName，自动生成
	if *wrapperName == "" {
		*wrapperName = extractedType + "Pool"
	}

	// 如果没有指定 outputPath，自动生成
	if *outputPath == "" {
		pkgName := strings.ToLower(extractedType) + "_pool"
		*outputPath = fmt.Sprintf("./generated/%s/client.go", pkgName)
	}

	// 创建生成器配置
	config := codegen.Config{
		PackagePath:      *packagePath,
		TypeName:         *typeName,
		WrapperName:      *wrapperName,
		PoolFieldName:    *poolFieldName,
		ClientType:       *clientType,
		OutputPath:       *outputPath,
		EnablePrometheus: *enablePrometheus,
	}

	// 创建生成器
	gen := codegen.NewGenerator(config)

	// 生成代码
	fmt.Printf("正在生成包装代码...\n")
	fmt.Printf("  源类型: %s.%s\n", config.PackagePath, config.TypeName)
	fmt.Printf("  包装器: %s\n", config.WrapperName)
	fmt.Printf("  输出文件: %s\n", config.OutputPath)

	if err := gen.Generate(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(config.OutputPath)
	fmt.Printf("\n✅ 代码生成成功!\n")
	fmt.Printf("   文件路径: %s\n", absPath)
}

// extractTypeName 从客户端类型字符串中提取类型名称
// 例如: "*rpc.Client" -> "Client", "codegen.It" -> "It", "*pkg.MyType" -> "MyType"
func extractTypeName(clientType string) string {
	// 移除指针符号
	clientType = strings.TrimPrefix(clientType, "*")

	// 如果包含包名前缀，取最后一部分
	parts := strings.Split(clientType, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return clientType
}
