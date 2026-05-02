package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"

	elfpacker "github.com/vmpacker/pkg/binary/elf"
)

// ============================================================
// vmpacker - ARM64/ARM32 ELF VMP 保护工具 (模块化版本)
//
// 用法:
//   vmpacker -func check_license [-v] [-o output] input.elf
//   vmpacker -info input.elf
//
// 功能:
//   读取编译好的 ARM64/ARM32 ELF，解码指定函数的指令，
//   翻译为自定义 VM 字节码，替换原函数为 VM 跳板。
// ============================================================

//go:embed vm_interp.bin
var interpBlob []byte

//go:embed vm_interp_arm32.bin
var interpBlobARM32 []byte

func main() {
	funcList := flag.String("func", "", "要保护的函数名（逗号分隔多个）")
	addrList := flag.String("addr", "", "按地址保护（格式: 0xADDR:SIZE[:name], 逗号分隔多个）")
	output := flag.String("o", "", "输出文件路径（默认: 原文件名.vmp）")
	verbose := flag.Bool("v", false, "详细输出（显示反汇编）")
	strip := flag.Bool("strip", true, "清除符号表（防止strip破坏保护）")
	debug := flag.Bool("debug", false, "生成 debug 对照文件（ARM64 → VM 字节码映射）")
	tokenEntry := flag.Bool("token", true, "启用 Token 化入口模式（3 指令跳板）— 默认开启")
	info := flag.Bool("info", false, "仅打印 ELF 信息，不做保护")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `vmpacker - ARM64/ARM32 ELF VMP 保护工具

用法:
  vmpacker -func <函数名> [-v] [-o output] <input.elf>
  vmpacker -addr <地址:大小[:名称]> [-v] [-o output] <input.elf>
  vmpacker -info <input.elf>

选项:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
示例:
  vmpacker -func check_license -v -o protected.elf original.elf
  vmpacker -func check_license -token -v -o protected.elf original.elf
  vmpacker -func "check_license,verify_token" app.elf
  vmpacker -addr "0x4006AC-0x400790" app.elf
  vmpacker -addr "0x4006AC-0x400790:main" -func verify app.elf
  vmpacker -info app.elf
`)
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	inputPath := flag.Arg(0)

	// 检查输入文件是否存在
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!] 文件不存在: %s\n", inputPath)
		os.Exit(1)
	}

	// 仅显示信息
	if *info {
		if err := elfpacker.PrintELFInfo(inputPath); err != nil {
			fmt.Fprintf(os.Stderr, "[!] %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 需要指定函数
	if *funcList == "" && *addrList == "" {
		fmt.Fprintf(os.Stderr, "[!] 请用 -func 或 -addr 指定要保护的函数\n")
		flag.Usage()
		os.Exit(1)
	}

	// 解析函数名列表
	var funcs []string
	if *funcList != "" {
		for _, f := range strings.Split(*funcList, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				funcs = append(funcs, f)
			}
		}
	}

	// 解析地址列表
	var addrSpecs []elfpacker.AddrSpec
	if *addrList != "" {
		for _, spec := range strings.Split(*addrList, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			as, err := elfpacker.ParseAddrSpec(spec)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[!] 地址格式错误: %s — %v\n", spec, err)
				os.Exit(1)
			}
			addrSpecs = append(addrSpecs, as)
		}
	}

	// 输出路径
	outPath := *output
	if outPath == "" {
		outPath = inputPath + ".vmp"
	}

	// 执行
	fmt.Println("========================================")
	fmt.Println("  vmpacker - ARM64/ARM32 ELF VMP 保护工具")
	fmt.Println("========================================")
	fmt.Printf("[*] 输入: %s\n", inputPath)
	fmt.Printf("[*] 输出: %s\n", outPath)
	fmt.Printf("[*] 保护函数: %v\n", funcs)
	fmt.Println()

	packer := elfpacker.NewPacker(inputPath, outPath, funcs, addrSpecs, *verbose, *strip, *debug, *tokenEntry, interpBlob)
	packer.SetInterpBlobARM32(interpBlobARM32)
	if err := packer.Process(); err != nil {
		fmt.Fprintf(os.Stderr, "\n[!] 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n[+] VMP 保护完成!")
}
