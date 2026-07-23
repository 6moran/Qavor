# 开发指南

本文档介绍如何在 Qavor 框架基础上开发新功能。

## 添加新功能流程

### 步骤 1: 定义数据模型

在 `internal/model/entity/` 创建新的实体文件：

```go
// internal/model/entity/product.go
package entity

type Product struct {
    BaseEntity
    Name        string  `gorm:"type:varchar(100);not null" json:"name"`
    Description string  `gorm:"type:text" json:"description"`
    Price       float64 `gorm:"type:decimal(10,2)" json:"price"`
    Stock       int     `gorm:"type:int" json:"stock"`
}

func (Product) TableName() string {
    return "products"
}
```

### 步骤 2: 创建 DTO

在 `internal/model/dto/request/` 和 `internal/model/dto/response/` 创建请求和响应结构：

```go
// internal/model/dto/request/product.go
package request

type CreateProductRequest struct {
    Name        string  `json:"name" binding:"required"`
    Description string  `json:"description"`
    Price       float64 `json:"price" binding:"required,gt=0"`
    Stock       int     `json:"stock" binding:"gte=0"`
}

type UpdateProductRequest struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Price       float64 `json:"price"`
    Stock       int    `json:"stock"`
}
```

```go
// internal/model/dto/response/product.go
package response

type ProductResponse struct {
    ID          uint    `json:"id"`
    Name        string  `json:"name"`
    Description string  `json:"description"`
    Price       float64 `json:"price"`
    Stock       int     `json:"stock"`
}
```

### 步骤 3: 创建 Repository

创建 `internal/repository/product_interface.go` 定义接口：

```go
type ProductRepository interface {
    FindByID(id uint) (*entity.Product, error)
    List(offset, limit int) ([]*entity.Product, int64, error)
    Create(product *entity.Product) error
    Update(product *entity.Product) error
    Delete(id uint) error
}
```

在 `internal/repository/product_repository.go` 实现接口：

```go
package repository

type productRepository struct {
    db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
    return &productRepository{db: db}
}

func (r *productRepository) FindByID(id uint) (*entity.Product, error) {
    var product entity.Product
    err := r.db.First(&product, id).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &product, nil
}

// 实现其他方法...
```

### 步骤 4: 创建 Service

创建 `internal/service/product_interface.go` 定义接口：

```go
type ProductService interface {
    Create(req *request.CreateProductRequest) error
    GetByID(id uint) (*entity.Product, error)
    Update(id uint, req *request.UpdateProductRequest) error
    Delete(id uint) error
    List(page, pageSize int) ([]*entity.Product, int64, error)
}
```

在 `internal/service/product_service.go` 实现业务逻辑：

```go
package service

type productService struct {
    productRepo repository.ProductRepository
}

func NewProductService(productRepo repository.ProductRepository) ProductService {
    return &productService{productRepo: productRepo}
}

func (s *productService) Create(req *request.CreateProductRequest) error {
    product := &entity.Product{
        Name:        req.Name,
        Description: req.Description,
        Price:       req.Price,
        Stock:       req.Stock,
    }
    return s.productRepo.Create(product)
}

// 实现其他方法...
```

### 步骤 5: 创建 Controller

在 `internal/api/v1/product/` 创建控制器：

```go
// internal/api/v1/product/controller.go
package product

type ProductController struct {
    productService service.ProductService
}

func NewProductController(productService service.ProductService) *ProductController {
    return &ProductController{productService: productService}
}

func (ctrl *ProductController) Create(c *gin.Context) {
    var req request.CreateProductRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, err.Error())
        return
    }

    if err := ctrl.productService.Create(&req); err != nil {
        response.BizError(c, err)
        return
    }

    response.Success(c, nil)
}

func (ctrl *ProductController) GetByID(c *gin.Context) {
    id := c.Param("id")
    productID, _ := strconv.ParseUint(id, 10, 32)

    product, err := ctrl.productService.GetByID(uint(productID))
    if err != nil {
        response.BizError(c, err)
        return
    }

    response.Success(c, product)
}

// 实现其他方法...
```

在 `internal/api/v1/product/routes.go` 注册路由：

```go
// internal/api/v1/product/routes.go
package product

func (ctrl *ProductController) RegisterRoutes(r *gin.RouterGroup) {
    productGroup := r.Group("/product")
    {
        productGroup.POST("", ctrl.Create)
        productGroup.GET("/:id", ctrl.GetByID)
        productGroup.PUT("/:id", ctrl.Update)
        productGroup.DELETE("/:id", ctrl.Delete)
    }
}
```

### 步骤 6: 注册路由

在 `internal/api/router.go` 中注册新路由：

```go
type Router struct {
    userCtrl    *user.Controller
    authCtrl    *auth.Controller
    productCtrl *product.ProductController  // 新增
}

func NewRouter(
    userService service.UserService,
    authService service.AuthService,
    productService service.ProductService,  // 新增
) *Router {
    return &Router{
        userCtrl:    user.NewController(userService),
        authCtrl:    auth.NewController(authService, userService),
        productCtrl: product.NewProductController(productService),  // 新增
    }
}

func (r *Router) Setup(engine *gin.Engine) {
    // ... 其他路由

    // 新增产品路由
    r.productCtrl.RegisterRoutes(v1)
}
```

### 步骤 7: 更新应用初始化

由于项目采用了应用启动器模式，添加新功能的初始化逻辑需要修改 `internal/app/app.go`：

在 `initDependencies()` 方法中添加新的 Repository 和 Service：

```go
// internal/app/app.go

func (a *App) initDependencies() {
    // 创建 Repository
    userRepo := repository.NewUserRepository(a.mysqlDB)
    productRepo := repository.NewProductRepository(a.mysqlDB)  // 新增

    // 创建 Service
    userSvc := service.NewUserService(userRepo)
    authSvc := service.NewAuthService(userRepo, userSvc)
    productSvc := service.NewProductService(productRepo)  // 新增

    // 创建 Router（传入所有 Service）
    a.router = api.NewRouter(userSvc, authSvc, productSvc)  // 更新
}
```

在 `initDatabase()` 方法的 AutoMigrate 中添加新实体：

```go
func (a *App) initDatabase() error {
    // ... 数据库连接代码

    // 自动迁移数据库表
    logger.Info("开始数据库迁移...")
    if err := a.mysqlDB.AutoMigrate(
        &entity.User{},
        &entity.Product{},  // 新增
    ); err != nil {
        logger.Warn("数据库迁移警告", zap.Error(err))
    } else {
        logger.Info("数据库迁移完成")
    }

    return nil
}
```

在 `internal/api/router.go` 中注册新路由（参见步骤 6）。

**优势**:
- main.go 不需要修改，保持简洁（约 20 行）
- 所有初始化逻辑集中在 `internal/app/app.go`
- 添加新功能只需要更新对应的方法
- 清晰的职责分离

## 代码规范

### 命名规范

- 文件名：小写，使用下划线分隔（如 `user_service.go`）
- 包名：小写单词
- 接口名：以能力命名，通常以 -er 结尾（如 `UserRepository`）
- 常量：大写，使用下划线分隔（如 `MAX_RETRY_COUNT`）

### 注释规范

```go
// UserService 用户服务接口
type UserService interface {
    // Register 用户注册
    // req: 注册请求信息
    // error: 注册失败返回错误
    Register(req *request.RegisterRequest) error
}
```

### 错误处理

使用自定义错误类型：

```go
// 使用预定义错误
if user == nil {
    return errors.ErrUserNotFound
}

// 创建自定义错误
if exists {
    return errors.New(errors.CodeUserAlreadyExists, "用户已存在")
}
```

## 测试

### 单元测试

```go
// internal/service/user_service_test.go
package service_test

func TestUserService_Register(t *testing.T) {
    // 使用 Mock Repository
    mockRepo := &MockUserRepository{}
    svc := service.NewUserService(mockRepo)

    // 测试用例
    err := svc.Register(&request.RegisterRequest{
        Username: "test",
        Password: "123456",
        Email:    "test@example.com",
    })

    assert.NoError(t, err)
}
```

## 调试技巧

### 1. 查看日志

```bash
tail -f logs/app.log
```

### 2. 打印调试信息

```go
logger.Debug("用户信息", zap.Any("user", user))
```

### 3. 数据库查询日志

在开发模式下，GORM 会自动打印 SQL 日志。

## 最佳实践

1. **保持层次清晰**: Controller → Service → Repository，不要跨层调用
2. **使用接口**: 定义接口便于测试和扩展
3. **错误处理**: 统一使用自定义错误类型
4. **日志记录**: 关键操作记录日志
5. **参数验证**: 在 Controller 层使用 validator 标签验证
6. **事务管理**: 在 Service 层使用 GORM 事务
