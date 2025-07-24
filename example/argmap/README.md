# Argmap 功能示例

这个示例展示了如何使用 `//goverter:argmap` 语法将函数参数直接赋值给目标结构体字段。

## 语法

```go
// goverter:argmap $<参数索引> <目标字段名>
```

- `$<参数索引>`: 参数索引，从1开始计数（$1, $2, $3, ...）
- `<目标字段名>`: 目标结构体中要赋值的字段名

## 示例

```go
// goverter:converter
type Converter interface {
    // goverter:argmap $2 TargetField2
    // goverter:argmap $3 TargetField3
    Convert(source Input, arg2 string, arg3 int) Output
}
```

在这个例子中：
- 第2个参数 `arg2` 会被直接赋值给 `Output.TargetField2`
- 第3个参数 `arg3` 会被直接赋值给 `Output.TargetField3`

## 生成的代码

```go
func (c *ConverterImpl) Convert(source Input, arg2 string, arg3 int) Output {
    var output Output
    output.Field1 = source.Field1           // 正常的字段映射
    output.TargetField2 = arg2              // argmap: $2 -> TargetField2
    output.TargetField3 = arg3              // argmap: $3 -> TargetField3
    return output
}
```

## 特性

1. **与其他映射兼容**: argmap 可以与 `goverter:map` 等其他映射指令一起使用
2. **类型安全**: 参数类型必须与目标字段类型兼容
3. **错误检查**: 如果参数索引超出范围，会在编译时报错
4. **上下文支持**: 支持与上下文参数一起使用

## 错误处理

- 如果参数索引超出范围，会显示错误：`argmap index $N is out of range, method has M arguments`
- 如果语法错误（如缺少$符号），会显示相应的语法错误信息