<div dir="rtl">

# gortisan

[English](README.md) | **فارسی**

[![test](https://github.com/alinemone/gortisan/actions/workflows/test.yml/badge.svg)](https://github.com/alinemone/gortisan/actions/workflows/test.yml)

یک تولیدکنندهٔ کد برای سرویس‌های HTTP در Go روی [echo v5](https://echo.labstack.com)، با حال‌وهوای `artisan` لاراول — اما بدون هیچ فریم‌ورکی پشتش.

یک ساختار پروژه که طوری انتخاب شده که **مبتدی هیچ‌وقت مجبور نباشد تصمیم بگیرد فایل کجا برود**، و آن‌قدر محکم است که تیمی با صد جدول و سه نسخهٔ API هم از آن بزرگ‌تر نشود. gortisan فایل‌ها را می‌نویسد و کنار می‌رود: پروژه **هیچ چیزی از gortisan ایمپورت نمی‌کند**، gortisan **در `go.mod` پروژه نیست**، و در زمان اجرا هیچ کدی اجرا نمی‌شود که خودتان نتوانید در مخزن خودتان بخوانید.

> **وضعیت:** آزمایشی (`v0.0.x`). دستورها و قالب‌ها هنوز ممکن است تغییر کنند.

- [نصب](#نصب)
- [شروع سریع](#شروع-سریع)
- [ساختار پروژه](#ساختار-پروژه)
- [اگر از لاراول می‌آیید](#اگر-از-لاراول-میآیید)
- [اسکریپت `./gortisan`](#اسکریپت-gortisan)
- [دستورها](#دستورها)
- [نسخه‌های API](#نسخههای-api)
- [لایهٔ دیتابیس](#لایهٔ-دیتابیس)
- [migration و seeder](#migration-و-seeder)
- [اعتبارسنجی و خطاها](#اعتبارسنجی-و-خطاها)
- [اضافه کردن auth، Sentry و…](#اضافه-کردن-authsentry-و)
- [بدون مجیک: قانون‌ها](#بدون-مجیک-قانونها)
- [وابستگی‌ها](#وابستگیها)
- [هنوز پشتیبانی نمی‌شود](#هنوز-پشتیبانی-نمیشود)
- [توسعه](#توسعه)

## نصب

به Go نسخهٔ ۱.۲۶ یا بالاتر نیاز دارد.

</div>

```bash
go install github.com/alinemone/gortisan/cmd/gortisan@latest
gortisan version
```

<div dir="rtl">

## شروع سریع

</div>

```bash
gortisan create shop --module github.com/you/shop
cd shop
cp .env.example .env          # آدرس PostgreSQL و Redis را بدهید
./gortisan run                # build، اجرا، و ری‌استارت با هر تغییر
```

```bash
curl localhost:8080/ping      # {"ping":"pong"}
curl localhost:8080/health    # {"status":"pass","checks":{"postgres":"up","redis":"up"}}
```

<div dir="rtl">

کاربران را با یک endpoint لیست اضافه کنید:

</div>

```bash
./gortisan make:model user --fields "mobile:string,name:string" --soft-delete
./gortisan migrate
./gortisan make:service user
./gortisan make:handler v1/user/list
```

<div dir="rtl">

هر دستور چیزی را که باید دستی وصل کنید چاپ می‌کند — یک خط در `v1/routes.go`، یک خط در `main.go` — و gortisan هیچ‌وقت کدی را که خودتان نوشته‌اید بازنویسی نمی‌کند.

## ساختار پروژه

</div>

```
shop/
├── cmd/
│   ├── api/main.go               سرویس HTTP: همهٔ وابستگی‌ها را می‌سازد و اجرا می‌کند
│   ├── migrate/main.go           اجرای migrationها
│   └── seed/main.go              اجرای seederها
├── internal/
│   ├── domain/                   کسب‌وکار — بدون HTTP
│   │   ├── user/
│   │   │   ├── user.go           نوع User: جدول، یک struct با تگ db
│   │   │   ├── errors.go         ErrNotFound و…: خطاها به زبان کسب‌وکار
│   │   │   ├── store.go          Store: آنچه سرویس از ذخیره‌سازی لازم دارد
│   │   │   ├── repository.go     Repository: پیاده‌سازی Store؛ نگاشت جدول آخرش ساخته می‌شود
│   │   │   ├── service.go        Service: قانون‌ها — تنها ورودی برای بقیه
│   │   │   └── status.go …       enum، validator، cache        (make:enum user/status …)
│   │   └── sales/                وقتی موضوع‌ها زیاد شد گروه‌بندی کنید
│   │       ├── order/
│   │       └── payment/
│   ├── api/                      HTTP — یک ورودی به domain
│   │   ├── router.go             ‎/ping و ‎/health و یک خط برای هر نسخه
│   │   ├── middleware/
│   │   ├── v1/
│   │   │   ├── routes.go         همهٔ endpointهای v1 در یک فایل
│   │   │   ├── users/            handler.go  request.go  response.go
│   │   │   └── sales/orders/
│   │   └── v2/                   فقط چیزهایی که در v2 عوض شده
│   ├── platform/                 زیرساخت — بدون قانون کسب‌وکار
│   │   ├── config/  server/  validate/  health/
│   │   ├── database/  database/orm/  database/migrate/
│   │   ├── cache/
│   │   └── payment/              کلاینت سرویس‌های دیگر           (make:client)
│   └── seed/                     seederها                        (make:seeder)
│   └── arch_test.go              اگر لایه‌ای جهت اشتباه import کند، go test خطا می‌دهد
├── migrations/                   SQL خام، ‎*.up.sql و ‎*.down.sql (داخل باینری embed می‌شوند)
├── gortisan, gortisan.bat, gortisan.ps1
└── go.mod                        فقط echo، pgx، go-redis — همان‌هایی که انتخاب کردید
```

<div dir="rtl">

**نام‌گذاری.** پکیج domain مفرد است — `user`، شامل `user.User` و `user.Service`. پکیج HTTP هم‌نام URL است — `‎/v1/users` در `internal/api/v1/users`، شامل `users.Handler` و `users.ListRequest`. این دو هیچ‌وقت با هم تداخل ندارند، پس هیچ importی alias لازم ندارد.

**چرا این شکل.** از [ساختار سرور تیم Go](https://go.dev/doc/modules/layout#server-project) پیروی می‌کند (دستورها در `cmd/` و بقیه در `internal/`) و از [راهنمای سبک Google](https://google.github.io/styleguide/go/best-practices#package-size) دربارهٔ پکیج‌ها: کدی که با هم استفاده می‌شود کنار هم است (همهٔ `user` در یک پکیج، در فایل‌های متمرکز)، و نام‌ها اسم پکیج را تکرار نمی‌کنند (`user.Service`). لایه‌های بالایی — همان تقسیمی که [پروژهٔ service از Ardan Labs](https://github.com/ardanlabs/service) دارد — باعث می‌شوند با بزرگ شدن پروژه هم همین شکل کار کند:

- **بیست موضوع یا دویست موضوع**، همه در `domain/` هستند و هیچ‌وقت با `config` و `database` قاطی نمی‌شوند؛ وقتی یک پوشه شلوغ شد گروه‌بندی کنید (`sales/order`).
- **domain نمی‌داند چه کسی صدایش می‌زند.** HTTP (`api/`)، یک consumer صف که خودتان اضافه می‌کنید (`worker/`)، یک CLI یا بعدها gRPC — همه همان `user.Service` را صدا می‌زنند.
- **وابستگی‌ها یک‌طرفه‌اند** و یک تست آن را اجبار می‌کند:

</div>

```
api, worker  ──►  domain  ──►  platform
```

<div dir="rtl">

`internal/arch_test.go` خطا می‌دهد اگر domain پکیج `api` را import کند، اگر `platform` به domain وابسته شود، یا اگر `api/v2` پکیج `api/v1` را import کند.

**فایل‌هایی که به آن‌ها اضافه می‌شود.** فایل‌هایی که یک نوع تعریف را جمع می‌کنند — `handler.go`، `request.go`، `response.go`، `validator.go`، `cache.go` — با اولین دستوری که لازمشان دارد ساخته می‌شوند و دستورهای بعدی به آن‌ها اضافه می‌کنند: `make:handler v1/user/get` یک متد به `handler.go`، نوع `GetRequest` به `request.go` و `GetResponse` به `response.go` اضافه می‌کند، همراه importهای لازم. هیچ چیزی از محتوای قبلی فایل عوض نمی‌شود؛ اگر اسمی از قبل وجود داشته باشد دست‌نخورده می‌ماند و گزارش می‌شود.

## اگر از لاراول می‌آیید

| لاراول | gortisan |
|---|---|
| `app/Models/User.php` | `internal/domain/user/user.go` |
| یک کلاس Service | `internal/domain/user/service.go` |
| `app/Http/Controllers/Api/V1/UserController.php` | `internal/api/v1/users/handler.go` |
| `app/Http/Requests/…Request.php` | `internal/api/v1/users/request.go` |
| `app/Http/Resources/UserResource.php` | `internal/api/v1/users/response.go` |
| `routes/api.php` | `internal/api/v1/routes.go` |
| `app/Http/Middleware` | `internal/api/middleware/` |
| `config/` | `internal/platform/config/` (و `.env`) |
| `database/migrations` | `migrations/` |
| `database/seeders` | `internal/seed/` |
| `php artisan serve` | `./gortisan run` |
| `php artisan make:model User -m` | `./gortisan make:model user` |
| `php artisan make:controller` | `./gortisan make:handler v1/user/list` |
| `php artisan migrate` / `migrate:rollback` / `db:seed` | `./gortisan migrate` / `migrate:rollback` / `db:seed` |

## اسکریپت `./gortisan`

هر پروژه در ریشه‌اش `gortisan`، `gortisan.bat` و `gortisan.ps1` دارد، درست مثل `artisan`:

</div>

```bash
./gortisan list                 # macOS / Linux / Git Bash
gortisan.bat list               # cmd
.\gortisan.ps1 list             # PowerShell
```

<div dir="rtl">

`artisan` در Goravel دستور `go run . artisan` را اجرا می‌کند، پس پروژه باید خود فریم‌ورک را import کند. اسکریپت gortisan دستور `go run github.com/alinemone/gortisan/cmd/gortisan@<version>` را اجرا می‌کند که روی همان نسخه‌ای که پروژه را ساخته پین شده است: همهٔ اعضای تیم کد یکسان تولید می‌کنند، کسی لازم نیست gortisan را نصب کند، و `go.mod` دست نمی‌خورد.

## دستورها

| دستور | کار |
|---|---|
| `create <name>` | پروژهٔ جدید در `./<name>` |
| `run [command] [-- args]` | `cmd/api` (یا `cmd/<command>`) را build و اجرا می‌کند، نگاشت جدول‌ها را دوباره می‌سازد و با هر تغییر در فایل `.go`، `go.mod` یا `.env` ری‌استارت می‌کند؛ `--no-watch` یک بار اجرا می‌کند |
| `generate` | نگاشت جدول هر struct دارای `//gortisan:table` را در `repository.go` آن می‌نویسد؛ `--check` اگر قدیمی باشد خطا می‌دهد (برای CI) |
| `make:model <subject>` | `user.go`، `errors.go`، `store.go` و `repository.go` در `internal/domain/<subject>`، و یک migration ساخت جدول *(postgres)* |
| `make:service <subject>` | `service.go`، ساخته‌شده روی `Store` اگر وجود داشته باشد |
| `make:handler <version>/<subject>/<action>` | یک متد در `internal/api/<version>/<resources>/handler.go`، ورودی‌اش در `request.go` و خروجی‌اش در `response.go`؛ نسخهٔ جدید `routes.go` خودش را می‌گیرد |
| `make:request <version>/<subject>/<name>` | یک نوع با `Validate` در `request.go` |
| `make:response <version>/<subject>/<name>` | یک نوع در `response.go` |
| `make:validator <subject>/<name>` | یک قانون قابل استفادهٔ مجدد (`validate.Rule`) در `validator.go` همان domain |
| `make:enum <subject>/<name>` | فایل `<name>.go` در domain: ثابت‌ها، `String`، `Valid`، `Parse…` و فهرست مقادیر |
| `make:cache <subject>/<name>` | یک cache تایپ‌شدهٔ Redis دور یک loader در `cache.go` همان domain *(redis)* |
| `make:middleware <name>` | `internal/api/middleware/<name>.go` |
| `make:client <name>` | `internal/platform/<name>/client.go`، کلاینت JSON برای یک سرویس دیگر |
| `make:migration <name>` | فایل‌های خالی `migrations/<time>_<name>.{up,down}.sql` |
| `make:test <subject>` | `service_test.go`: تست سرویس با یک `Store` درون‌حافظه‌ای، بدون دیتابیس |
| `make:test <version>/<subject>/<action>` | `<action>_test.go`: تست HTTP همان handler |
| `add <postgres\|redis>` | بخشی را که پروژه بدون آن ساخته شده اضافه می‌کند: فایل‌ها، config، خط‌های `.env.example` و ماژول؛ دو خطی را که باید وصل کنید چاپ می‌کند |
| `make:seeder <name>` | `internal/seed/<name>.go`، و `cmd/seed` بار اول |
| `migrate` | migrationهای اجرانشده را در یک batch اعمال می‌کند |
| `migrate:rollback [batches]` | آخرین batch (یا چند batch) را برمی‌گرداند |
| `migrate:status` | همهٔ migrationها و اینکه اجرا شده‌اند یا نه |
| `migrate:fresh --force` | همهٔ جدول‌ها را حذف و از نو migrate می‌کند — فقط برای محیط توسعه |
| `db:seed [name]` | seederها را در یک تراکنش اجرا می‌کند |
| `list`، `help`، `version` | |

موضوع‌ها می‌توانند گروه‌بندی شوند — `make:model sales/order` و `make:handler v1/sales/order/list`. `make handler` (با فاصله) همان `make:handler` است. راهنمایی که برای handler چاپ می‌شود مسیر REST را از اسم action حدس می‌زند: `list` ← `GET /users`، `get` ← `GET /users/:id`، `create` ← `POST /users`، `update` ← `PUT /users/:id`، `delete` ← `DELETE /users/:id`.

**فلگ‌ها**

| فلگ | دستور | معنی |
|---|---|---|
| `--module <path>` | `create` | مسیر ماژول Go (پیش‌فرض: اسم پروژه) |
| `--with <list>` | `create` | `postgres`، `redis`، `all` (پیش‌فرض) یا `none` |
| `--no-tidy` | `create` | `go mod tidy` را اجرا نکن |
| `--no-watch` | `run` | فقط یک بار اجرا کن |
| `--fields "<list>"` | `make:model` | اختیاری: ستون‌ها، مثل `mobile:string,name:string` |
| `--table <name>` | `make:model` | اسم جدول (پیش‌فرض: جمع اسم مدل) |
| `--soft-delete` | `make:model` | ستون `deleted_at`؛ حذف آن را پر می‌کند و کوئری‌ها آن ردیف‌ها را نمی‌بینند |
| `--members "<list>"` | `make:enum` | `pending,paid=5,sent`: اعداد از ۱ یا مقدار قبلی ادامه پیدا می‌کنند |
| `--type <type>` | `make:enum` | `string` یا یک نوع عدد صحیح (پیش‌فرض `int64`) |
| `--type <type>` | `make:cache` | نوع مقدار cache، مثل `int64` یا `*User` (پیش‌فرض `string`) |
| `--force` | `make:*` | فایل‌های موجود را کامل بازنویسی کن |

**نوع فیلدها** برای `--fields`: `string`/`text`، `int`/`int64`/`bigint`، `int32`/`integer`، `int16`، `float`/`float64`، `bool`، `time`، `json`/`jsonb`؛ یک `?` در انتها ستون را nullable می‌کند.

## نسخه‌های API

هر نسخه یک پکیج است: handlerها، requestها و responseهای خودش، با domain مشترک بین همهٔ نسخه‌ها. v1 کار می‌کند در حالی که v2 تغییر می‌کند.

</div>

```go
// internal/api/v1/routes.go — همهٔ endpointهای v1 در یک جا
type Services struct {
	User  *user.Service
	Order *order.Service
}

func Register(g *echo.Group, s Services) {
	userHandler := users.NewHandler(s.User)
	g.GET("/users", userHandler.List)
	g.GET("/users/:id", userHandler.Get)

	orderHandler := orders.NewHandler(s.Order)
	g.GET("/orders", orderHandler.List)
}
```

```go
// cmd/api/main.go — سیم‌کشی، جلوی چشم
users := user.NewService(user.NewRepository(db))
orders := order.NewService(order.NewRepository(db))

api.Register(e, api.Routes{
	Health: health.NewHandler(db, rdb),
	V1:     v1.Services{User: users, Order: orders},
	V2:     v2.Services{User: users},
})
```

<div dir="rtl">

`./gortisan make:handler v2/user/list` بار اول `internal/api/v2/routes.go` را می‌سازد و خطی را که باید در `internal/api/router.go` اضافه کنید چاپ می‌کند. v2 فقط چیزهایی را دارد که عوض شده؛ تست معماری نمی‌گذارد v1 را import کند.

## لایهٔ دیتابیس

struct تنها جایی است که یک ستون تعریف می‌شود — مثل تگ‌های یک ORM، با این فرق که **هنگام generate** خوانده می‌شود، نه در زمان اجرا:

</div>

```go
// internal/domain/user/user.go
//gortisan:table users
type User struct {
	ID        int64      `db:"id,pk" json:"id"`
	Mobile    string     `db:"mobile" json:"mobile"`
	Name      string     `db:"name" json:"name"`
	Note      string     `json:"note"`                         // بدون تگ db: ستون نیست
	CreatedAt time.Time  `db:"created_at,default" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at,default" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at,default" json:"deleted_at,omitempty"`
}
```

<div dir="rtl">

`gortisan generate` نگاشت را در انتهای `repository.go` همان پکیج، بین دو خط نشانه می‌نویسد — کد ساده، با ستون‌ها، `Scan` و فهرست insert به‌صراحت. بقیهٔ فایل دست نمی‌خورد:

</div>

```go
// gortisan:table User begin — generated from user.go by gortisan generate; edits here are overwritten.
var userTable = orm.Table[User]{
	Name:       "users",
	PrimaryKey: "id",
	Columns:    []string{"id", "mobile", "name", "created_at", "updated_at", "deleted_at"},
	Scan: func(row orm.Scanner, m *User) error {
		return row.Scan(&m.ID, &m.Mobile, &m.Name, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt)
	},
	InsertColumns:    []string{"mobile", "name"},
	InsertValues:     func(m *User) []any { return []any{m.Mobile, m.Name} },
	UpdatedAtColumn:  "updated_at",
	SoftDeleteColumn: "deleted_at",
}
// gortisan:table User end
```

<div dir="rtl">

| در struct | معنی |
|---|---|
| `//gortisan:table name` | این struct یک جدول است |
| `db:"name"` | یک ستون؛ خوانده و insert می‌شود |
| `db:"id,pk"` | کلید اصلی: خوانده می‌شود، هیچ‌وقت insert نمی‌شود |
| `db:"created_at,default"` | دیتابیس پرش می‌کند: بعد از insert خوانده می‌شود، insert نمی‌شود |
| بدون تگ `db` یا `db:"-"` | ستون نیست |
| ستونی به اسم `updated_at` / `deleted_at` | در update / soft delete برابر `now()` می‌شود |

برای اضافه کردن ستون، یک فیلد و یک migration اضافه کنید. `make:model` و `gortisan run` نگاشت را برایتان دوباره می‌سازند؛ در CI، `gortisan generate --check` اگر نگاشتی قدیمی باشد خطا می‌دهد. هیچ چیزی در زمان اجرا struct را بررسی نمی‌کند.

`Service` به `Store` وابسته است، یک interface در `store.go` که `Repository` پیاده‌اش می‌کند — پس تست سرویس می‌تواند یک store جعلی بدهد و بدون دیتابیس اجرا شود. repository خطای `ErrNotFound` خود domain را برمی‌گرداند، نه خطای orm را؛ پس هیچ چیز بالای آن به ذخیره‌سازی وابسته نیست. `internal/platform/database/orm` داخل پروژهٔ خودتان است؛ از آن استفاده کنید، SQL دستی بنویسید، یا هر دو را با هم:

</div>

```go
u, err := userTable.Find(ctx, r.db, id)
users, err := userTable.Get(ctx, r.db,
	orm.Where("name ILIKE ?", "%ali%").Where("mobile LIKE ?", "0912%").OrderBy("id DESC").Limit(20))

var n int64
err := r.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE created_at >= $1`, since).Scan(&n)

err := database.InTx(ctx, db, func(tx pgx.Tx) error { // تراکنش یک مقدار است و صریح پاس داده می‌شود
	if err := users.WithTx(tx).Create(ctx, u); err != nil {
		return err
	}
	return payments.WithTx(tx).Create(ctx, p)
})
```

<div dir="rtl">

## migration و seeder

</div>

```bash
./gortisan make:model user                 # migrations/<time>_create_users_table.{up,down}.sql را هم می‌سازد
./gortisan make:migration add_index_to_users
./gortisan migrate                         # اعمال migrationهای اجرانشده
./gortisan migrate:status
./gortisan migrate:rollback                # برگرداندن آخرین batch
./gortisan make:seeder users && ./gortisan db:seed
```

<div dir="rtl">

اجراکنندهٔ migration بخشی از پروژهٔ خودتان است — `internal/platform/database/migrate`، چند صد خط روی pgx — و `cmd/migrate` یک دستور معمولی است که در production قبل از اینکه نسخهٔ جدید ترافیک بگیرد build و اجرایش می‌کنید. فایل‌های SQL داخل باینری embed می‌شوند (`migrations/migrations.go`)، پس image به چیز دیگری نیاز ندارد.

- migrationهای اعمال‌شده با یک شمارهٔ **batch** در `schema_migrations` ثبت می‌شوند؛ `migrate:rollback` مثل لاراول batchها را کامل، از جدیدترین، برمی‌گرداند.
- هر migration همراه ثبتش **در یک تراکنش** اجرا می‌شود. فایلی که خط `-- migrate:no-transaction` دارد بیرون از تراکنش اجرا می‌شود؛ برای `CREATE INDEX CONCURRENTLY`.
- یک **advisory lock در Postgres** باعث می‌شود فقط یک پروسه در هر لحظه migrate کند؛ پس replicaهایی که هم‌زمان بالا می‌آیند هیچ migrationی را دو بار اجرا نمی‌کنند.
- دو `make:migration` در یک ثانیه نسخه‌های متفاوت می‌گیرند.

seederها تابع‌هایی در `internal/seed` هستند که یک `database.DBTX` می‌گیرند و از repositoryهای domain استفاده می‌کنند؛ `cmd/seed` آن‌ها را به ترتیب فهرست می‌کند و در یک تراکنش اجرا می‌کند (`./gortisan db:seed users` فقط یکی را اجرا می‌کند).

## اعتبارسنجی و خطاها

یک request همهٔ مشکلاتش را جمع می‌کند و با هم برمی‌گرداند؛ هر چیزی که یک بررسی لازم دارد مثل بقیه پاس داده می‌شود:

</div>

```go
// internal/api/v1/users/request.go
func (r *CreateRequest) Validate(ctx context.Context, users *user.Service) error {
	var errs validate.Errors
	if len(r.Mobile) != 11 {
		errs.Add("mobile", "must be 11 digits")
	}
	if err := errs.Check(ctx, "mobile", user.NewMobileFree(users, r.Mobile)); err != nil {
		return err // خود بررسی نتوانست اجرا شود
	}
	return errs.Err()
}
```

```json
422 {"message": "The given data was invalid.",
     "errors": [{"field": "mobile", "message": "must be 11 digits"}]}
```

<div dir="rtl">

domain خطاها را به زبان کسب‌وکار برمی‌گرداند (`errors.go`)؛ handler تصمیم می‌گیرد کد HTTP چه باشد:

</div>

```go
u, err := h.service.Find(ctx, req.ID)
if errors.Is(err, user.ErrNotFound) {
	return echo.NewHTTPError(http.StatusNotFound, "user not found")
}
if err != nil {
	return err // یک 500؛ جزئیاتش به لاگ می‌رود، نه به کلاینت
}
```

<div dir="rtl">

بدنهٔ هر خطا `{"message": "..."}` است. هر درخواست یک خط لاگ JSON با method، مسیر، status و مدت زمان می‌نویسد؛ خطاهای 5xx همراه علتشان لاگ می‌شوند و panic همراه stack.

سرور یک `http.Server` ساده است با `ReadHeaderTimeout`، `ReadTimeout`، `WriteTimeout`، `IdleTimeout` و `MaxHeaderBytes` (با `HTTP_*` در `.env`)، پس یک کلاینت کند یا بیکار نمی‌تواند اتصال‌ها را باز نگه دارد. بازیابی از panic چند خط در `internal/platform/server` است، نه یک کتابخانه.

## اضافه کردن auth، Sentry و…

gortisan عمداً این‌ها را تولید نمی‌کند: هر شرکت آن‌ها را جور دیگری انجام می‌دهد و نسخهٔ تولیدشده مجبور می‌شد کتابخانه‌ای را به‌جای شما انتخاب کند. هر کدام چند خط است، دستی اضافه می‌شود و صریح می‌ماند:

</div>

```bash
./gortisan make:middleware authenticate
```

```go
// internal/api/middleware/authenticate.go
func Authenticate(tokens *auth.Client) echo.MiddlewareFunc { // SDK احراز هویت خودتان
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			who, err := tokens.Verify(c.Request().Context(), c.Request().Header.Get("Authorization"))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			c.Set("caller", who) // یا صریح پاسش دهید
			return next(c)
		}
	}
}

// internal/api/v1/routes.go
g.Use(middleware.Authenticate(s.Tokens))
```

<div dir="rtl">

گزارش خطا (Sentry و مشابهش) هم همین‌طور: یک middleware که به `server.New(logger, sentryMiddleware)` داده می‌شود، یا یک فراخوانی در error handler پکیج `server`.

## بدون مجیک: قانون‌ها

1. **هیچ importی از تولیدکننده نیست.** فقط پکیج‌های خود پروژه و کتابخانه‌هایی که انتخاب کرده‌اید. تست‌های gortisan این را در هر build بررسی می‌کنند.
2. **وابستگی‌ها پاس داده می‌شوند، جست‌وجو نمی‌شوند.** constructorها چیزی را که لازم دارند می‌گیرند و `main.go` آن‌ها را به هم وصل می‌کند؛ هیچ چیز در `context.Context` ذخیره یا از آن خوانده نمی‌شود.
3. **لایه‌ها یک‌طرفه‌اند:** `api` و `worker` ← `domain` ← `platform`. `internal/arch_test.go` این را در هر پروژه اجبار می‌کند.
4. **در کد پروژه reflection نیست.** config هر متغیر را با اسمش می‌خواند؛ نگاشت جدول‌ها کد تولیدشده است. (`c.Bind` در echo تنها جایی است که یک کتابخانه از reflection استفاده می‌کند، و می‌توانید جایگزینش کنید.)
5. **تراکنش‌ها مقدارند** — `pgx.Tx` که به `repo.WithTx(tx)` داده می‌شود — و **خطاها معنی دارند**: domain اسمشان را می‌گذارد و API آن‌ها را به status تبدیل می‌کند.

## وابستگی‌ها

پروژهٔ تولیدشده به **echo v5** وابسته است و اگر انتخابشان کنید به **pgx** (PostgreSQL) و **go-redis**. هیچ چیز دیگر: Kafka، متریک، گزارش خطا (Sentry…)، tracing (OpenTelemetry…)، دیتابیس‌ها یا صف‌های دیگر را هر وقت لازم داشتید خودتان اضافه کنید — یک middleware، یک کلاینت در `platform/`، یک خط در `main.go`. یک starter kit که SDKهای دیگران را پین کند، با تغییر آن‌ها می‌شکند.

## هنوز پشتیبانی نمی‌شود

- `route:list`، مستندات OpenAPI / Swagger

## توسعه

</div>

```bash
go test ./...          # برای هر انتخاب --with یک پروژه می‌سازد، همهٔ make:*ها را اجرا می‌کند،
                       # بعد go mod tidy، go vet و go test (ماژول‌ها را دانلود می‌کند)
go test -short ./...   # فقط تست‌های واحد
```

<div dir="rtl">

قالب‌ها در `internal/generator/templates` هستند و داخل باینری embed می‌شوند. CI کل تست‌ها را برای هر push و pull request روی Linux و Windows اجرا می‌کند.

انتشار نسخه با تگ git است: `git tag -a v0.0.7 -m v0.0.7 && git push origin v0.0.7`.

</div>
