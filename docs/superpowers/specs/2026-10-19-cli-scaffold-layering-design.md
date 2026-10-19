# sicky CLI 脚手架分层对齐设计

- 日期：2026-10-19
- 参考项目：`../../Herewe/axisos/svc/svc.key-cabinet`
- 范围：`cli/`（`sicky new`、`sicky generate model`、`sicky generate handler`、模板与测试）
- 状态：待实现

## 1. 意图（Why）

`sicky new` 目前只生成 `main.go`、`config.go`、`config.json`、`handler/<transport>.go` 四个文件。真实业务服务 `svc.key-cabinet` 在同样的扁平根目录上多出一整套约定：

- `app/`：应用级配置（`ConfigDef` + `Ensure()` + 全局 `Config`）与内存 session；
- `model/`：bun 模型（`Create/Get/List/Count/Update/Delete/String/Validate/InitDB`）；
- `service/`：业务层；
- `handler/`：基础 handler（`ping` + 鉴权中间件）、每资源 `<resource>.http.go`、`request/` 与 `response/` DTO 及响应码。

结果是：用 sicky 新建项目后，开发者仍需手工抄一遍参考项目的目录和样板代码。这个设计把参考项目的结构、命名、响应封装、文件头格式固化进脚手架，并让“模型”成为引入 bun/数据库的唯一开关。

## 2. 目标（Goals）

1. `sicky new --type standard` 在选择 HTTP 时生成与参考项目一致的完整分层（`app/`、`handler/`、`handler/request/`、`handler/response/`、`service/`）。
2. 默认不引入 bun、不连接数据库；生成的项目在没有任何外部依赖时即可 `make build` 并 `make run`。
3. `sicky generate model <name>` 一条命令生成 model + service + handler + request/response DTO，并自动完成 bun 接线（`config.json` 的 `infra.bun`、`main.go` 的 `MustInfra` / `--initdb` / `InitDB` 列表 / handler 注册）。
4. `sicky new --model a,b` 在创建项目时直接生成上述全链路。
5. 命名与格式对齐参考项目：`HTTP<Resource>` / `NewHTTP<Resource>` / `Name() = "<resource>.http"`，文件头含 `@brief`，保留 vim modeline。
6. 生成的 `go.mod` 指向 Go 1.27，模型使用标准库 `uuid`（参考项目约定）。

## 3. 非目标（Non-goals）

- 不修改 `mcp` / `interactive` 项目类型的文件结构（共享的头部/格式改动除外）。
- 不生成 Fiber / gRPC 版本的 auth 示例；auth 示例只走 net/http + bunrouter。
- 不生成数据库迁移工具；建表仍由 `InitDB` 的 `IfNotExists` 完成。
- 不在生成过程中执行 `go mod tidy`、`go get` 或任何网络请求。
- 不生成任何出厂凭据（`admin/admin`、固定 token 等）。

## 4. 反目标（Anti-goals）

- `sicky new` 在默认路径下不能要求数据库存在。
- `sicky generate model` 不能在无法理解目标文件时“猜着改”；必须给出明确错误并且不留下半修改状态。
- 生成的代码不能出现框架明令禁止的写法：未检查的错误、日志消息大写、`context.TODO()`。

## 5. 已确认的决策

| # | 决策 |
|---|------|
| D1 | 完整分层模板，standard 类型适用。 |
| D2 | 没有确认的 model 就不生成 bun；model 是 bun 的唯一开关。 |
| D3 | `sicky generate model` 自动接线（config.json + main.go），并一次生成全套资源文件。 |
| D4 | 生成 auth/session 示例；默认用户名/密码/token 全部为空，不提供 `admin/admin`。 |
| D5 | 默认服务组合不硬改，按 `sicky new` 的交互/标志选择；只有选中 HTTP 时才生成 HTTP 分层与 auth 示例。 |
| D6 | request/response DTO 按参考项目聚合在 `handler/request/http.go` 与 `handler/response/http.go`；后续 `generate model` 向这两个文件追加声明。 |
| D7 | CRUD 处理器与参考项目 `key.http.go` 一致，先给路由 + 空实现 + TODO；auth 是真正可运行的示例。 |
| D8 | 模板目录改为镜像输出路径，`go:embed` 改为递归嵌入整个 `template/project` 目录。 |
| D9 | `@brief` 加入文件头；每个模板提供一句描述。 |
| D10 | 生成 `AGENTS.md` 与更新后的 `README.md`（结构、命令、模型接线说明）。 |

## 6. 生成的项目结构（standard + HTTP，无 model）

```
<project>/
  main.go
  config.go
  config.json
  app/
    config.go
    session.go
  handler/
    http.go              # HTTPHttp：ping + validateSession + getToken
    auth.http.go         # HTTPAuth：login / logout
    request/http.go      # DTO
    response/http.go     # Code*/Msg* + 响应 DTO
    fiber.go             # 仅在 --fiber 时生成（保持现状）
    grpc.go              # 仅在 --grpc 时生成（保持现状）
  service/
    auth.go
  Makefile
  Dockerfile
  .dockerignore
  .gitignore
  README.md
  AGENTS.md
  go.mod
```

有 model 时追加：

```
  model/<name>.go
  service/<name>.go
  handler/<name>.http.go
  # request/http.go、response/http.go 中追加该资源的 DTO
  # config.json 增加 sicky.infra.bun
  # main.go 增加 MustInfra/--initdb/initdb()/handler 注册
```

`--no-auth` 时移除 `app/session.go`、`handler/auth.http.go`、`service/auth.go`，`handler/http.go` 不再包含 `validateSession`/`getToken`，`main.go` 不注册 `NewHTTPAuth()`。资源 handler（`handler/<name>.http.go`）此时不挂 `validateSession`，只保留 `router.NewGroup("/<name>")`；`--model` 与 `--no-auth` 允许同时使用，产品必须可编译。

`--http` 未选中时不生成 HTTP 分层文件（`handler/http.go`、`handler/auth.http.go`、`request/`、`response/`、`service/auth.go`），其余文件照常；此时 `--model` 是非法组合（见 8.1）。

## 7. 文件规格

### 7.1 `main.go`

对齐参考项目，但错误必须检查：

```go
var (
    AppName   = "{{.AppName}}"
    Version   = "latest"
    Branch    = "main"
    Commit    = ""
    BuildTime = ""
)

func main() {
    ctx := context.Background()

    if err := sicky.Init(
        &sicky.Options{
            AppName:   AppName,
            Version:   Version,
            Branch:    Branch,
            Commit:    Commit,
            BuildTime: BuildTime,
            Context:   ctx,
            MustInfra: []string{"bun"}, // 仅在有 model 时
        },
        &sicky.FlagSwitch{ // 仅在有 model 时
            Flag:     "initdb",
            Usage:    "Initialize the database tables and exit",
            Callback: initdb,
        },
    ); err != nil {
        if errors.Is(err, sicky.ErrVersionShown) {
            return
        }

        log.Fatalf("sicky: init: %v", err)
    }
    if err := sicky.ConfigUnmarshal(&config); err != nil {
        log.Fatalf("sicky: config unmarshal: %v", err)
    }

    app.Config = config.App.Ensure()

    httpSrv := srvHTTP.New(...)
    httpSrv.Handle(handler.NewHTTPHttp())
    httpSrv.Handle(handler.NewHTTPAuth())        // --no-auth 时省略
    httpSrv.Handle(handler.NewHTTPKey())         // 每个 model
    ...
}
```

有 model 时追加：

```go
func initdb() error {
    ctx := context.Background()

    if infra.Bun == nil {
        logger.Logger.FatalContext(ctx, "database connection is not initialized")
    }

    if err := (model.Key{}).InitDB(ctx, false); err != nil {
        return err
    }
    // 每个 model 一行

    os.Exit(0)

    return nil
}
```

> `os.Exit(0)` 与 `return nil` 同时存在是参考项目的写法；保留它以便 `--initdb` 成功后立即退出进程，不进入 `sicky.Run`。

### 7.2 `config.go` / `config.json`

```go
type ConfigDef struct {
    Server struct {
        HTTP *srvHTTP.Config `json:"http" yaml:"http" mapstructure:"http"`
        // fiber/grpc 按选择出现
    } `json:"server" yaml:"server" mapstructure:"server"`
    Service *standard.Config `json:"service" yaml:"service" mapstructure:"service"`
    Sicky   *sicky.Config    `json:"sicky" yaml:"sicky" mapstructure:"sicky"`
    App     *app.ConfigDef   `json:"app" yaml:"app" mapstructure:"app"`
}
```

`config.json` 在现有内容上增加 `app` 段：

```json
"app": {
    "admin_username": "",
    "admin_password": "",
    "general_token": ""
}
```

有 model 时增加：

```json
"sicky": {
    "infra": {
        "bun": {
            "driver": "postgres",
            "dsn": "postgres://postgres:postgres@localhost:5432/{{.Name}}?sslmode=disable"
        }
    }
}
```

DSN 是占位值，README 中说明必须替换；不在模板里放任何真实口令。

### 7.3 `app/config.go`

对齐参考项目：`ConfigDef`、`Ensure()`、包级 `Config`。默认值全部为空字符串（D4），`Ensure()` 只做 nil 保护与字段归一化：

```go
type ConfigDef struct {
    AdminUsername string `json:"admin_username" mapstructure:"admin_username" yaml:"admin_username"`
    AdminPassword string `json:"admin_password" mapstructure:"admin_password" yaml:"admin_password"`
    GeneralToken  string `json:"general_token"  mapstructure:"general_token"  yaml:"general_token"`
}

func (c *ConfigDef) Ensure() *ConfigDef {
    if c == nil {
        c = &ConfigDef{}
    }

    return c
}

var Config *ConfigDef
```

### 7.4 `app/session.go`

照搬参考项目：`sync.Map`、`AccessToken`、`AppendSession`、`DeleteSession`、`CheckSession`，token 前缀 `sess_`，使用 `utils.RandomString(32)`。

### 7.5 `handler/http.go`

```go
type HTTPHttp struct{}

func NewHTTPHttp() *HTTPHttp
func (h *HTTPHttp) Name() string { return "http.http" }
func (h *HTTPHttp) Type() string { return "http" }
func (h *HTTPHttp) Register(router *bunrouter.Router) { router.GET("/ping", h.ping) }

func (h *HTTPHttp) ping(...) error { return bunrouter.JSON(...) } // 或 utils.BunAutoFormat + WrapHTTPResponse
```

`validateSession`：

- 读取 `Authorization`；缺失 → `CodeInvalidCertificate` + 401。
- `strings.Fields` 后必须是 2 段且第一段为 `Bearer`；否则 `CodeInvalidAuthorization` + 401。
- token 匹配：`app.Config.GeneralToken` 使用 `crypto/subtle.ConstantTimeCompare`（长度不同直接失败），或命中 `app.CheckSession(token)`。
- 成功后把 token 放进 request context（`ctxKeyToken`），`getToken` 从 context 取。
- 日志消息小写，错误细节只进服务端日志。
- `--no-auth` 时整个 `validateSession`/`getToken`/`ctxKeyToken` 块不生成；依赖它的资源 handler 也不挂中间件（见 §8.2 的检测规则）。

### 7.6 `handler/auth.http.go`

对齐参考项目：

- `Register`：`POST /auth/login` 公开，`POST /auth/logout` 走 `validateSession`。
- `login`：`utils.BunAutoBind` → `service.Auth.Authenticate` → `app.AppendSession` → `utils.BunAutoFormat`；失败返回 401 + `CodeInvalidCertificate`。
- `logout`：`app.DeleteSession(getToken(req.Request))`，返回 bool。
- 全部错误响应走 `utils.BunAutoFormatWithStatus`。

### 7.7 `handler/request/http.go` 与 `handler/response/http.go`

`request/http.go`：

```go
type HttpAuthPostLogin struct {
    Username string `json:"username" form:"username"`
    Password string `json:"password" form:"password"`
}

type HttpAuthPostLogout struct{}
```

`response/http.go`：参考项目的 `Code*` / `Msg*` 常量与 `HttpAuthPostLogin/Logout` 响应 DTO。注意 `MsgOK` 在参考项目里是 `0`（int），这会与 `utils.MsgOK`（string）冲突；生成时改用字符串 `"OK"`，并让所有 `Message` 字段保持 string 类型。这是对参考项目的修正，避免类型不一致。

每个 model 追加：

```go
// request
type HttpKeyPostCreate struct { Name string `json:"name" form:"name"` }
type HttpKeyPutUpdate  struct { Name string `json:"name" form:"name"` }

// response
type HttpKey struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}
```

DTO 只使用内建类型，避免追加声明时还要修改 import 块。

### 7.8 `service/auth.go`

```go
type Auth struct{}

func (s *Auth) Authenticate(username, password string) error {
    if app.Config.AdminUsername == "" || app.Config.AdminPassword == "" {
        return errors.New("authentication is not configured")
    }

    if subtle.ConstantTimeCompare([]byte(username), []byte(app.Config.AdminUsername)) == 1 &&
        subtle.ConstantTimeCompare([]byte(password), []byte(app.Config.AdminPassword)) == 1 {
        return nil
    }

    return errors.New("invalid credentials")
}
```

> 与参考项目的 `admin/admin` 默认值不同（D4）。未配置时登录明确失败，不产生“默认可用”的假象。

### 7.9 `model/<name>.go`

对齐参考项目 `model/key.go` 的骨架，但字段按生成名推导：

- `bun.BaseModel` + `bun:"table:<lower(name)>"`。
- `ID uuid.UUID`（标准库 `uuid`，`bun:"id,pk,default:uuid_generate_v4()"`）。
- `Name string`（占位字段，带 `// TODO: replace with real fields.` 注释）。
- `CreatedAt` / `UpdatedAt time.Time`。
- `TableName`、`String`、`Validate`。
- `Create` / `Get` / `List` / `Count` / `Update` / `Delete`（使用 `infra.Bun`、`logger.Logger`、`utils.ListFilter`，与参考项目一致）。
- `InitDB(ctx, dropTable)`：建表 + `name`、`created_at`、`updated_at` 索引。
- 表名/索引名做小写与下划线归一化；拒绝不能构成合法标识符的名称（沿用 `validateSchematicName`）。
- `uuid_generate_v4()` 是 PostgreSQL 的 `uuid-ossp` 函数。`initdb()` 在第一个模型 `InitDB` 之前执行 `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`（`infra.Bun.ExecContext`），否则干净数据库上的 `--initdb` 会失败。非 PostgreSQL 驱动需自行去掉该 DDL 默认值；README 说明这一点。

### 7.10 `service/<name>.go` 与 `handler/<name>.http.go`

- `service/<name>.go`：`type <Name> struct{}`，与参考项目 `service/key.go` 一样留空，后续填业务。
- `handler/<name>.http.go`：`HTTP<Name>` / `NewHTTP<Name>` / `Name() "<name>.http"` / `Type() "http"`。
  - `Register`：`router.NewGroup("/<name>")`；项目有 auth（`handler/http.go` 中存在 `validateSession`）时追加 `.Use(validateSession)`。路由 `GET /list`、`POST /`、`GET /:id`、`PUT /:id`、`DELETE /:id`。
  - 处理器为空实现 + TODO（D7），但必须使用 `req`/`w` 参数避免编译错误；可直接 `return nil`。

### 7.11 `AGENTS.md` / `README.md`

`AGENTS.md` 参考项目式内容：

- 项目简介与当前状态；
- `make` 目标（build/test/docker）；
- 运行方式与端口；
- 有 model 时的 `--initdb` 说明（双横线）；
- 分层约定（每资源文件、DTO 命名、响应封装、`uuid` 使用标准库）；
- 配置位置（`app/` vs 根 `config.go`）。

`README.md` 更新 `Project Structure`，并说明 `--model` / `generate model` / auth 默认凭据为空。

### 7.12 `Makefile`

生成的 `go.mod` 只有 `module`/`go` 指令，没有 `require`。`go build` 在 Go 1.16+ 默认 `-mod=readonly`，缺依赖会直接失败。为避免“新建项目后 `make build` 不能跑”，共享 Makefile 增加：

```make
deps:
	@$(GO) mod tidy
```

并让 `all: summary proto deps fmt build`、`build: deps`、`test: deps`。`go mod tidy` 的首次执行需要网络，这是正常 Go 工作流；脚手架本身仍然不联网（§3 非目标不变）。README/AGENTS 说明首次构建会下载依赖。

## 8. CLI 变更

### 8.1 `sicky new`

新增标志：

- `--model strings`：逗号分隔（pflag `StringSlice`），可重复。每个名字生成一套 model/service/handler/DTO 并启用 bun 接线。
- `--no-auth`：跳过 auth/session 示例。
- `--http` 默认值与交互提示保持一致：交互提示当前显示 `[Y/n]`，但标志默认是 false；把提示改成 `[y/N]`，避免“回车即开启”与默认值矛盾（D5，按交互选择执行）。

`newContext` 增加：

```go
Models  []modelSpec // Name, ExportedName, Table
WithAuth bool
```

模板嵌入改为：

```go
//go:embed template/project template/license/*.gotmpl
var projectTemplates embed.FS
```

并把模板目录改成镜像输出路径：

```
template/project/standard/app/config.go.gotmpl
template/project/standard/app/session.go.gotmpl
template/project/standard/handler/http.go.gotmpl
template/project/standard/handler/auth.http.go.gotmpl
template/project/standard/handler/request/http.go.gotmpl
template/project/standard/handler/response/http.go.gotmpl
template/project/standard/service/auth.go.gotmpl
template/project/standard/model/model.go.gotmpl
template/project/standard/service/resource.go.gotmpl
template/project/standard/handler/resource.http.go.gotmpl
template/project/standard/AGENTS.md.gotmpl
```

`scaffoldProject` 的文件清单从静态列表改为“静态列表 + 按 model 循环渲染”。所有 `.go` 文件仍走现有 `injectFileHeader` / `injectFileFooter` / `format.Source` 流程。

`--model` 与 `--no-auth` 的交互约束：

- `--model` 非空要求 HTTP 已选中（`--http` 或交互确认）。否则在创建前报错：`--model requires an HTTP server (--http)`。
- `--model` 非空 → `WithAuth` 默认仍为 true（登录示例与 model 无冲突）。
- `--no-auth` 只影响 auth 相关文件与资源 handler 的中间件；model 文件照常生成，资源 handler 不挂 `validateSession`。

### 8.2 `sicky generate model <name>`

帮助与注册：

```
sicky generate model <name> [--output <dir>] [--force] [--no-wire]
```

适用前提（不满足时在写任何文件之前报错）：

- 项目是 `sicky new --type standard` + HTTP 的产物：存在 `handler/http.go`、`handler/request/http.go`、`handler/response/http.go`。
- `main.go` 存在且包含 `sicky.Init`、`&sicky.Options{}`；否则报 `main.go does not look like a sicky scaffold: <missing anchor>`。
- 没有 `httpSrv := srvHTTP.New(...)` 时不阻止执行：生成文件与 bun 接线，打印警告并跳过 handler 注册。
- 旧版 4 文件项目（没有 `handler/request|response`）不自动补层；报错并提示先用 `sicky new --http` 重建，或手工创建这两个文件。

行为：

1. 读取当前项目 `go.mod` 得到模块路径；读取 `main.go`、`handler/http.go` 做接线预检，并检测 `validateSession` 是否存在（决定资源 handler 是否挂中间件）。
2. 生成：
   - `model/<name>.go`
   - `service/<name>.go`
   - `handler/<name>.http.go`
   - 向 `handler/request/http.go` 追加该资源的 DTO 声明
   - 向 `handler/response/http.go` 追加该资源的响应 DTO 声明
3. 接线（除非 `--no-wire`）：
   - `config.json`：若 `sicky.infra.bun` 不存在则补上（占位 DSN，见 7.2）。
   - `main.go`：
     - 保证 import 中有 `<module>/model`；
     - `sicky.Options` 增加 `MustInfra: []string{"bun"}`（已有则去重）；
     - `sicky.Init` 增加 `&sicky.FlagSwitch{Flag: "initdb", ...}`（已有则不重复）；
     - 保证 `initdb()` 存在，并追加 `if err := (model.<Name>{}).InitDB(ctx, false); err != nil { return err }`；
     - 若存在 `httpSrv := srvHTTP.New(...)`，追加 `httpSrv.Handle(handler.NewHTTP<Name>())`；不存在则警告并跳过。
4. 所有接线先预检、再写文件、最后原子替换；预检失败时不做任何修改。
5. `--no-wire`：只写文件，打印需要手工接线的三处提示（`config.json` 的 `infra.bun`、`main.go` 的 `MustInfra`/`--initdb`/`InitDB` 列表、handler 注册）。
6. 重复执行语义：
   - 不带 `--force`：目标资源文件已存在则报 `exists (use --force)`，整次命令不写任何文件、不改任何接线。
   - 带 `--force`：覆盖 `model/<name>.go`、`service/<name>.go`、`handler/<name>.http.go`；聚合 DTO 文件按类型名去重（已存在 `HttpKey*` 声明就跳过），不产生重复声明；`main.go`/`config.json` 的接线按“已存在就跳过”去重。
   - 因此 `TestGenerateModelWiresProject` 的第二次执行必须带 `--force`。

接线实现要点：

- `main.go` 用 `go/parser` + `go/ast` 定位 `main` 函数、`sicky.Init` 调用、`&sicky.Options{}` 复合字面量、`initdb` 函数、`httpSrv.Handle` 语句；用 `go/format` 输出。找不到 `sicky.Init`/`Options` 时返回：`main.go does not look like a sicky scaffold: <missing anchor>`。
- `config.json` 用 `encoding/json` 解码为 `map[string]any`，补键后用 `json.MarshalIndent(..., "", "    ")` 写回；改写后键按字母序排列，这是可接受的（该文件由脚手架管理）。
- DTO 追加用 `go/parser` 解析片段并把 `Decls` 追加到目标文件，再 `go/format`；片段不允许带 import（DTO 只用内建类型），若未来需要 import 则在此处显式处理。
- 所有写文件用 `os.CreateTemp` + `os.Rename` 原子替换，避免半写状态。

### 8.3 `sicky generate handler`

- 保持现有 `--type fiber|http` 与文件路径规则。
- HTTP 模板命名改为参考项目风格：
  - 输入名 `key.http` + `--type http` → 文件 `handler/key.http.go`，类型 `HTTPKey`，`NewHTTPKey`，`Name() "key.http"`。
  - 输入名 `key` + `--type http` → 文件 `handler/key.go`，类型 `HTTPKey`，`Name() "key.http"`。
- Fiber 模板保持现状（`<Name>Fiber`），不引入 auth。

## 9. 格式规范

- 文件头：`Copyright` + `SPDX-License-Identifier`，随后 `@file/@package/@author/@since/@brief`；`@brief` 每个模板提供一句描述（D9）。
- vim modeline 保留在 `.go` 文件末尾。
- 命名：
  - 基础 HTTP handler：`HTTPHttp` / `NewHTTPHttp` / `"http.http"`。
  - 资源 HTTP handler：`HTTP<Resource>` / `NewHTTP<Resource>` / `"<resource>.http"`。
  - DTO：`Http<Resource><Method><Action>`（如 `HttpKeyPostCreate`）。
  - 模型：`<Resource>`（导出形式）。
- 日志：生成代码中的 `logger.Logger.*Context` 消息一律小写（框架 §3.8）。
- 空行：块后接代码留空行、`return` 前留空行（框架 §3.8）；生成后仍走 `gofmt`，但这两条不会被 gofmt 自动修复。
- 错误：所有可能失败的调用都检查；不生成裸 `_ =`。
- 安全：不生成出厂凭据；token 比较使用 `crypto/subtle`；`/config` 等框架端点保持既有默认。

## 10. 兼容与迁移

- `mcp` / `interactive`：目录结构不变；仅共享的 `@brief`、`go.mod` 版本、README/AGENTS 生成逻辑同步。
- 已有项目：`sicky generate handler` 的 HTTP 命名变化只影响新生成文件；不自动改写旧文件。
- 生成 `go.mod` 的 `go` 指令改为 `1.27.0`；model 使用标准库 `uuid`。这是参考项目的要求，也是本设计唯一提升工具链下限的地方。
- 不改变 `sicky new` 的默认服务器组合；交互提示与标志默认值对齐（HTTP 提示改为 `[y/N]`）。

## 11. 验证计划

### 11.1 单元/集成测试

1. `TestNewStandardLayering`：`sicky new demo --module example.com/demo --http --fiber=false --no-grpc` 后断言：
   - `app/config.go`、`app/session.go`、`handler/http.go`、`handler/auth.http.go`、`handler/request/http.go`、`handler/response/http.go`、`service/auth.go` 存在；
   - `handler/http.go` 含 `HTTPHttp` 与 `validateSession`；
   - `main.go` 含 `handler.NewHTTPHttp()` 与 `handler.NewHTTPAuth()`。
2. `TestNewWithoutAuth`：`--no-auth` 时上述 auth 文件不存在，`main.go` 不注册 `NewHTTPAuth()`；`--model key --no-auth` 时 `handler/key.http.go` 不包含 `.Use(validateSession)`，且生成的 `handler/http.go` 不含 `validateSession`。
3. `TestNewWithModels`：`--model key,cabinet` 时：
   - `model/key.go`、`service/key.go`、`handler/key.http.go` 等存在；
   - `config.json` 含 `sicky.infra.bun`；
   - `main.go` 含 `MustInfra`、`initdb`、`CREATE EXTENSION`、两个 `InitDB` 调用与两个 handler 注册。
4. `TestNewModelRequiresHTTP`：`--model key` 且未选 HTTP 时退出非 0，错误信息包含 `--model requires an HTTP server`。
5. `TestGenerateModelWiresProject`：在脚手架产物中运行 `sicky generate model train`，断言文件 + `config.json` + `main.go` 全部接线；再用 `--force` 跑一次断言不重复（DTO 类型只出现一次）。
6. `TestGenerateModelNoWire`：`--no-wire` 时 `config.json`/`main.go` 字节不变。
7. `TestGenerateModelMissingAnchors`：删除 `main.go` 的 `sicky.Init` 或 `sicky.Options` 后运行，断言退出非 0、错误信息包含缺失锚点、且 `model/train.go` 未被写入（预检先于写入）。
8. `TestGenerateModelOutsideLayering`：在 fiber-only 或旧版 4 文件项目中运行，断言退出非 0 且错误提示缺失的 `handler/request/http.go` 等文件，未写入任何文件。
9. `TestGenerateHandlerHTTPNaming`：`generate handler key.http --type http` 产出 `HTTPKey` / `NewHTTPKey` / `Name() "key.http"`。
10. `TestScaffoldBuilds`（`testing.Short()` 跳过）：在临时目录生成项目，写入 `replace github.com/go-sicky/sicky => <repoRoot>` 后执行 `go mod tidy` + `go build ./...`；网络不可用时 `t.Skip` 并说明原因。
11. 更新 `cli/scaffold_config_test.go`：新增 `app` 段、model 场景的 `infra.bun`、以及 `@brief` 的文件头断言。

### 11.2 负对照（仓库 §6.1 要求）

每个修复/新增测试都要有负对照：撤掉实现，确认测试因正确原因失败。重点：

- 去掉 `MustInfra` 接线 → `TestGenerateModelWiresProject` 失败且指向该字段；
- 去掉 `validateSession` 挂载 → auth 路由测试失败；
- 把默认凭据改回 `admin/admin` → `TestNewStandardLayering` 的“空默认值”断言失败；
- 去掉 `--no-auth` 分支 → 文件仍被生成，测试失败。

### 11.3 手工冒烟

```
sicky new demo --module example.com/demo --http --fiber=false --no-grpc
cd demo
make build           # 首次会执行 go mod tidy
./bin/svc            # /ping 可用；未配置凭据时 /auth/login 返回 401
sicky generate model key
make build
# 配置 config.json 的 app.* 与 infra.bun 后
./bin/svc --initdb   # 建表并退出（需要 PostgreSQL）
```

## 12. 留给实现者的决定

- `model` 的占位字段名与索引集合（本设计给 `Name` + `name/created_at/updated_at` 索引）可按实现便利微调，但必须保留 `ID/CreatedAt/UpdatedAt`。
- `generate model` 的接线实现细节（AST 遍历的具体锚点匹配）由实现者决定，但语义必须是“预检 → 写文件 → 原子替换”。
- `AGENTS.md` 的具体措辞。
- 是否把 `TestScaffoldBuilds` 纳入 `make verify`（默认建议纳入，`testing.Short()` 跳过网络依赖）。

## 13. Builder check 决议

以下问题由设计者在收到 builder check 后确定，已写回上文：

1. **`--no-auth` 与资源 handler 的矛盾**：`--no-auth` 删除 `validateSession`；`new` 通过 `WithAuth` 把“是否挂中间件”传给资源模板；`generate model` 解析 `handler/http.go` 检测 `validateSession` 是否存在。`--model` + `--no-auth` 合法且必须可编译。
2. **`go.mod` 依赖来源**：生成项目不在脚手架阶段联网；生成的 Makefile 增加 `deps: go mod tidy`，`all`/`build`/`test` 依赖它。`TestScaffoldBuilds` 在临时目录先 `go mod tidy` 再 `go build`，网络不可用时跳过。
3. **非 HTTP/旧版项目中的 model 生成**：`sicky new --model` 要求 HTTP；`generate model` 要求 `handler/http.go` + `handler/request/http.go` + `handler/response/http.go` 存在，否则预检失败、不写任何文件。没有 `httpSrv` 时只警告并跳过注册。
4. **重复执行语义**：不带 `--force` 遇到已存在资源文件即失败且不写任何东西；带 `--force` 覆盖资源文件、DTO 按类型名去重、接线按存在即跳过。
5. **PostgreSQL UUID 默认值**：保留参考项目的 `default:uuid_generate_v4()`；`initdb()` 在建表前执行 `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`。非 PostgreSQL 驱动需自行去掉该默认值，README 说明。
