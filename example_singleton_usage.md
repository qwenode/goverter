# 单例变量使用示例

这个功能为每个生成的 `xxxImpl` 结构体自动创建一个单例变量，方便直接调用。

## 示例

### 输入代码

```go
package example

// goverter:converter
type UserConverter interface {
    ConvertUser(source User) UserDTO
}

// goverter:converter  
type ProductServiceImpl interface {
    ConvertProduct(source Product) ProductDTO
}

type User struct {
    Name string
}

type UserDTO struct {
    Name string
}

type Product struct {
    Title string
}

type ProductDTO struct {
    Title string
}
```

### 生成的代码

```go
// Code generated by github.com/jmattheis/goverter, DO NOT EDIT.

package generated

import execution "github.com/jmattheis/goverter/execution"

type ProductServiceImplImpl struct{}

var ProductServiceImplConvert = ProductServiceImplImpl{}

func (c *ProductServiceImplImpl) ConvertProduct(source execution.Product) execution.ProductDTO {
    var executionProductDTO execution.ProductDTO
    executionProductDTO.Title = source.Title
    return executionProductDTO
}

type UserConverterImpl struct{}

var UserConverterConvert = UserConverterImpl{}

func (c *UserConverterImpl) ConvertUser(source execution.User) execution.UserDTO {
    var executionUserDTO execution.UserDTO
    executionUserDTO.Name = source.Name
    return executionUserDTO
}
```

### 使用方式

现在你可以直接使用生成的单例变量：

```go
package main

import (
    "fmt"
    "your-project/generated"
)

func main() {
    user := User{Name: "John"}
    userDTO := generated.UserConverterConvert.ConvertUser(user)
    fmt.Printf("Converted user: %+v\n", userDTO)
    
    product := Product{Title: "Laptop"}
    productDTO := generated.ProductServiceImplConvert.ConvertProduct(product)
    fmt.Printf("Converted product: %+v\n", productDTO)
    
    // 也可以直接使用，无需实例化
    order := OrderDTO{ID: 1, UserID: 123, Amount: 99.99}
    convertedOrder := generated.OrderConverterImplConvert.ConvertOrder(order)
    fmt.Printf("Converted order: %+v\n", convertedOrder)
}
```

## 命名规则

- 如果接口名以 `Impl` 结尾，单例变量名会去掉 `Impl` 后缀，然后添加 `Convert` 后缀
  - `UserConverterImpl` → `var UserConverterImplConvert = UserConverterImplImpl{}`
- 如果接口名不以 `Impl` 结尾，单例变量名与接口名相同，然后添加 `Convert` 后缀
  - `UserConverter` → `var UserConverterConvert = UserConverterImpl{}`
- 所有生成的变量名都是首字母大写的导出变量，方便外部包使用

这样设计使得调用更加直观和方便！