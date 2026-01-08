# 项目架构问题分析报告

> 基于 `cmd/gui/main.go` 入口的完整架构扫描

## 一、依赖注入与初始化混乱

### 1.1 循环依赖风险
**位置**: `internal/ui/app.go:48-70`

**问题描述**:
```go
// AppState 创建 Store 时传入 nil
dataStore := store.NewStore(nil)

// 然后创建 SubscriptionManager
subscriptionManager := subscription.NewSubscriptionManager()

// 再设置回 Store
dataStore.Subscriptions.SetSubscriptionManager(subscriptionManager)
```

**问题**:
- `Store` 在创建时无法获得完整的依赖，需要后续手动设置
- 违反了依赖注入原则，增加了初始化顺序的复杂性
- `Store` 和 `SubscriptionManager` 之间存在双向依赖

**建议**: 使用依赖注入容器或工厂模式，一次性完成所有依赖的初始化

---

### 1.2 初始化顺序不清晰
**位置**: `internal/ui/app.go:301-329` (Startup 方法)

**问题描述**:
```go
func (a *AppState) Startup() error {
    // 1. InitApp() - 创建 Fyne 应用，加载 Store 数据
    // 2. NewMainWindow() - 创建主窗口
    // 3. InitLogger() - 初始化日志（需要 LogsPanel）
    // 4. SetupTray() - 设置托盘
    // 5. SetupWindowCloseHandler() - 设置关闭事件
}
```

**问题**:
- 各步骤之间存在隐式依赖关系（如 Logger 需要 LogsPanel）
- 初始化失败时，已初始化的资源可能无法正确清理
- 缺少初始化状态检查

**建议**: 
- 明确各步骤的依赖关系，使用依赖图管理初始化顺序
- 添加初始化状态标记，防止重复初始化

---

## 二、数据访问层混乱

### 2.1 绕过 Store 直接访问数据库
**位置**: 
- `internal/ui/mainwindow.go:792` - `database.SetAppConfig`
- `internal/ui/mainwindow.go:801` - `database.GetAppConfig`
- `internal/ui/logs.go:65,196` - 直接访问 `database`
- `internal/ui/resources.go:85` - 直接访问 `database`

**问题描述**:
UI 层直接调用 `database` 包，绕过了 `Store` 层

**问题**:
- 违反了分层架构原则（UI -> Service -> Store -> Database）
- 数据更新不会触发 Store 的绑定更新
- 无法统一管理数据访问逻辑

**建议**: 
- 所有数据库操作应通过 `Store` 或 `Service` 层
- UI 层只应访问 `AppState.Store` 或服务层接口

---

### 2.2 Store 职责过重
**位置**: `internal/store/store.go`

**问题描述**:
`Store` 既负责数据访问，又负责业务逻辑（如订阅更新）

**问题**:
```go
// Store 中的 SubscriptionsStore 直接调用 SubscriptionManager
func (ss *SubscriptionsStore) UpdateByID(id int64) error {
    if ss.subscriptionManager == nil {
        return fmt.Errorf("订阅管理器未初始化")
    }
    // 调用业务逻辑
    if err := ss.subscriptionManager.UpdateSubscriptionByID(id); err != nil {
        return err
    }
    // 刷新数据
    return ss.Load()
}
```

**问题**:
- `Store` 应该只负责数据访问和绑定管理
- 业务逻辑（如订阅更新）应该在 `Service` 层

**建议**: 
- 将业务逻辑从 `Store` 移到 `Service` 层
- `Store` 只负责 CRUD 操作和数据绑定

---

### 2.3 Service 层不完整
**位置**: `internal/service/server.go`

**问题描述**:
- 只有 `ServerService`，缺少其他业务服务
- `ServerService` 有时直接访问 `database`，有时通过 `Store`

**问题**:
```go
// ServerService 直接访问 database
func (ss *ServerService) GetServersBySubscriptionID(subscriptionID int64) ([]database.Node, error) {
    servers, err := database.GetServersBySubscriptionID(subscriptionID)
    // ...
}

// 但有时又通过 Store
func (ss *ServerService) UpdateServerDelay(id string, delay int) error {
    return ss.store.Nodes.UpdateDelay(id, delay)
}
```

**问题**:
- 数据访问方式不统一
- 缺少 `SubscriptionService`、`ConfigService` 等

**建议**: 
- 统一 Service 层的数据访问方式（都通过 Store）
- 补充缺失的业务服务层

---

## 三、UI 与业务逻辑耦合

### 3.1 MainWindow 包含业务逻辑
**位置**: `internal/ui/mainwindow.go`

**问题描述**:
`MainWindow` 直接操作数据库和系统代理，包含大量业务逻辑

**问题**:
```go
// MainWindow 直接操作数据库
func (mw *MainWindow) saveSystemProxyState(mode string) {
    if err := database.SetAppConfig("systemProxyMode", mode); err != nil {
        // ...
    }
}

// MainWindow 直接操作系统代理
func (mw *MainWindow) applySystemProxyMode(fullModeName string) error {
    // 大量业务逻辑
}
```

**问题**:
- UI 组件不应该包含业务逻辑
- 难以测试和维护

**建议**: 
- 将系统代理管理逻辑移到 `Service` 层
- `MainWindow` 只负责 UI 展示和事件转发

---

### 3.2 AppState 职责不清
**位置**: `internal/ui/app.go:19-46`

**问题描述**:
`AppState` 既管理应用状态，又管理 UI 组件引用

**问题**:
```go
type AppState struct {
    // 状态管理
    PingManager *ping.PingManager
    Store *store.Store
    XrayInstance *xray.XrayInstance
    
    // UI 组件引用（不应该在这里）
    MainWindow *MainWindow
    LogsPanel *LogsPanel
}
```

**问题**:
- `AppState` 应该只管理应用状态，不应该持有 UI 组件引用
- 违反了关注点分离原则

**建议**: 
- 将 UI 组件引用移除，使用事件或回调机制通信
- `AppState` 只管理业务状态

---

## 四、包结构问题

### 4.1 包职责不清晰
**目录结构**:
```
internal/
  config/        # 配置定义（但实际配置在 database）
  database/      # 数据库访问 + 数据模型
  store/         # 数据访问 + 业务逻辑 + 绑定管理
  service/       # 服务层（不完整）
  ui/            # UI + 业务逻辑
  xray/          # xray 封装
  subscription/  # 订阅解析 + 业务逻辑
  ping/          # 延迟测试
```

**问题**:
- `config` 包存在但未使用（配置在 `database` 包）
- `store` 包混合了数据访问和业务逻辑
- `ui` 包混合了 UI 和业务逻辑
- `subscription` 包混合了解析和业务逻辑

**建议**: 
- 明确各包的职责边界
- 考虑重构为更清晰的分层结构

---

### 4.2 数据模型位置不当
**位置**: `internal/database/db_type.go`

**问题描述**:
数据模型定义在 `database` 包中，但被其他层广泛使用

**问题**:
- `database.Node` 和 `database.Subscription` 被 UI、Service、Store 等层直接使用
- 违反了依赖倒置原则（高层模块依赖低层模块的具体实现）

**建议**: 
- 将数据模型移到独立的 `model` 或 `domain` 包
- 各层通过接口访问数据模型

---

## 五、具体混乱位置汇总

### 5.1 直接访问 database 的位置

| 文件 | 行号 | 问题 |
|------|------|------|
| `internal/ui/mainwindow.go` | 792, 801 | 直接设置/获取系统代理配置 |
| `internal/ui/logs.go` | 65, 196 | 直接设置/获取日志折叠状态 |
| `internal/ui/resources.go` | 85 | 直接获取主题配置 |
| `internal/store/store.go` | 475, 508, 516, 521, 526 | Store 内部直接访问 database（可接受，但应统一） |

### 5.2 业务逻辑位置不当

| 文件 | 行号 | 问题 |
|------|------|------|
| `internal/ui/mainwindow.go` | 694-772 | 系统代理管理业务逻辑 |
| `internal/store/store.go` | 300-360 | 订阅更新业务逻辑 |
| `internal/ui/app.go` | 86-130 | 状态更新业务逻辑 |

### 5.3 依赖注入问题

| 文件 | 行号 | 问题 |
|------|------|------|
| `internal/ui/app.go` | 58, 67, 70 | Store 创建时传入 nil，后续手动设置依赖 |
| `internal/ui/app.go` | 301-329 | 初始化顺序复杂，依赖关系不清晰 |

---

## 六、架构改进建议

### 6.1 分层架构建议

```
┌─────────────────────────────────────┐
│         UI Layer (ui/)              │  ← 只负责 UI 展示和事件转发
├─────────────────────────────────────┤
│      Service Layer (service/)       │  ← 业务逻辑层
│  - ServerService                    │
│  - SubscriptionService (新增)       │
│  - ConfigService (新增)             │
│  - ProxyService (新增)               │
├─────────────────────────────────────┤
│      Store Layer (store/)           │  ← 数据访问和绑定管理
│  - NodesStore                       │
│  - SubscriptionsStore               │
│  - ConfigStore                      │
├─────────────────────────────────────┤
│   Database Layer (database/)        │  ← 数据库访问
├─────────────────────────────────────┤
│      Model Layer (model/)           │  ← 数据模型（新增）
└─────────────────────────────────────┘
```

### 6.2 依赖注入改进

**当前问题**:
```go
store := store.NewStore(nil)  // 传入 nil
manager := subscription.NewSubscriptionManager()
store.Subscriptions.SetSubscriptionManager(manager)  // 后续设置
```

**建议方案**:
```go
// 方案1: 使用构造函数注入
manager := subscription.NewSubscriptionManager()
store := store.NewStore(manager)

// 方案2: 使用依赖注入容器
container := NewContainer()
container.Register(store.NewStore)
container.Register(subscription.NewSubscriptionManager)
container.Wire()
```

### 6.3 统一数据访问

**当前问题**: UI 层直接访问 `database`

**建议**: 
- 所有数据访问通过 `Store` 或 `Service` 层
- UI 层只访问 `AppState.Store` 或服务接口

---

## 七、优先级建议

### 高优先级（影响架构清晰度）
1. ✅ 移除 UI 层对 `database` 的直接访问
2. ✅ 将业务逻辑从 `Store` 移到 `Service` 层
3. ✅ 改进依赖注入机制

### 中优先级（影响可维护性）
4. ✅ 将数据模型移到独立的 `model` 包
5. ✅ 补充缺失的 Service 层（SubscriptionService、ConfigService 等）
6. ✅ 将系统代理管理逻辑移到 Service 层

### 低优先级（代码质量）
7. ✅ 清理未使用的 `config` 包
8. ✅ 统一代码风格和注释

---

## 八、总结

当前架构的主要问题：
1. **分层不清晰**: UI、Service、Store 层职责混乱
2. **依赖注入混乱**: 初始化顺序复杂，存在循环依赖风险
3. **数据访问不统一**: 存在绕过 Store 直接访问数据库的情况
4. **业务逻辑位置不当**: UI 层和 Store 层包含业务逻辑

建议按照上述优先级逐步重构，建立清晰的分层架构。

