# 使用abi创建go包

## 1. 安装abigen

```go install github.com/ethereum/go-ethereum/cmd/abigen@latest```

## 2. 准备abi文件

- 文件必须是json格式，且内容直接以[ 开头，以 ] 结尾，没有外层 {}

## 3 执行命令

```bigen --abi .\abi\C2NSale.abi.json --pkg model --type C2NSale --out .\model\sale.go```

### 命令说明

- --abi：abi文件路径
- --pkg：生成的 Go 包名
- --type：生成的 Go 结构体名称
- --out：输出文件路径
  