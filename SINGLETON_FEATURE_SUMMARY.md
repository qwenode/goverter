# 单例变量功能实现总结

## 功能描述

为每个生成的 `xxxImpl` 结构体自动创建一个单例变量，方便外部调用。

## 实现细节

### 修改的文件
- `generator/generator.go` - 在 `appendGenerated` 方法中添加了单例变量生成逻辑

### 核心代码
```go
if g.conf.OutputFormat == config.FormatStruct {
    if len(g.conf.Comments) > 0 {
        f.Comment(strings.Join(g.conf.Comments, "\n"))
    }
    f.Type().Id(g.conf.Name).Struct()
    
    // 生成单例变量，例如: var ConverterConvert = ConverterImpl{}
    singletonName := strings.TrimSuffix(g.conf.Name, "Impl")
    if singletonName == g.conf.Name {
        // 如果名称不以 Impl 结尾，则使用原名称
        singletonName = g.conf.Name
    }
    // 确保首字母大写并添加 Convert 后缀
    if len(singletonName) > 0 {
        singletonName = strings.ToUpper(singletonName[:1]) + singletonName[1:] + "Convert"
    }
    f.Var().Id(singletonName).Op("=").Id(g.conf.Name).Values()
}
```

## 命名规则

1. **去除 Impl 后缀**：如果接口名以 `Impl` 结尾，单例变量名会去掉 `Impl` 后缀
2. **添加 Convert 后缀**：所有单例变量名都添加 `Convert` 后缀
3. **首字母大写**：确保变量名首字母大写，使其成为导出变量

### 示例
- `UserConverter` → `var UserConverterConvert = UserConverterImpl{}`
- `UserConverterImpl` → `var UserConverterImplConvert = UserConverterImplImpl{}`
- `OrderService` → `var OrderServiceConvert = OrderServiceImpl{}`
- `ProductServiceImpl` → `var ProductServiceImplConvert = ProductServiceImplImpl{}`

## 使用方式

```go
// 直接使用生成的单例变量
user := User{Name: "John", Email: "john@example.com"}
userDTO := generated.UserServiceConvert.CreateUser(user)

order := OrderDTO{ID: 1, UserID: 123, Amount: 99.99}
convertedOrder := generated.OrderConverterImplConvert.ConvertOrder(order)
```

## 优势

1. **方便调用**：无需手动实例化结构体
2. **导出变量**：可以被外部包使用
3. **一致命名**：统一的命名规则，易于理解和使用
4. **向后兼容**：不影响现有的结构体和方法生成

## 测试覆盖

创建了多个测试场景验证功能：
- `scenario/singleton_variable.yml` - 基本功能测试
- `scenario/singleton_variable_complex.yml` - 复杂场景测试
- `scenario/singleton_variable_edge_cases.yml` - 边界情况测试
- `scenario/singleton_real_world.yml` - 真实使用场景测试

所有测试都通过，确保功能正常工作且不影响现有功能。