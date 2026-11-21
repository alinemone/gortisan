<div dir="rtl">

# gortisan

[English](README.md) | **فارسی**

تولیدکنندهٔ کد برای سرویس‌های HTTP در Go روی [echo v5](https://echo.labstack.com)، با حال‌وهوای `artisan` لاراول، ولی بدون هیچ فریم‌ورکی پشتش.

یک ساختار feature-first برای همهٔ سرویس‌های تیم: دستورهای Cobra در `cmd/`، یک پکیج برای هر feature در `internal/`، و wiring در `internal/app`. gortisan فایل‌ها را می‌نویسد، وصلشان می‌کند و همان‌جا کارش تمام می‌شود: پروژه **gortisan را import نمی‌کند**، gortisan **در `go.mod` آن نیست**، و در زمان اجرای درخواست هیچ چیزی اجرا نمی‌شود که نتوانید در ریپوی خودتان بخوانید.

> **وضعیت:** پیش‌نمایش (`v0.x`). از `v1.0.0` به بعد هیچ نسخه‌ای پروژه‌های موجود را خراب نمی‌کند — [COMPATIBILITY.fa.md](COMPATIBILITY.fa.md) را ببینید.

- [نصب](#نصب)
- [شروع سریع](#شروع-سریع)
- [ساختار پروژه](#ساختار-پروژه)
- [دستورها](#دستورها)
- [featureها](#featureها)
- [migration](#migration)
- [Swagger](#swagger)
- [لایهٔ دیتابیس](#لایهٔ-دیتابیس)
- [اعتبارسنجی و خطاها](#اعتبارسنجی-و-خطاها)
- [دستورهای بیشتر: worker و schedule](#دستورهای-بیشتر-worker-و-schedule)
- [نسخه‌ها و به‌روزرسانی](#نسخه‌ها-و-به‌روزرسانی)
- [بدون جادو: قواعد](#بدون-جادو-قواعد)
- [توسعه](#توسعه)

## نصب

به Go نسخهٔ 1.26 یا بالاتر نیاز دارد.

</div>

```bash
go install github.com/alinemone/gortisan/cmd/gortisan@latest
gortisan version
```

<div dir="rtl">

### اگر Go نتوانست gortisan را پیدا کند

gortisan یک ماژول خصوصی روی GitLab شرکت است. اگر `go install` یا اسکریپت `./gortisan` یک پروژه گفت ماژول پیدا نشد (`404`، `unknown revision` یا `terminal prompts disabled`)، یک بار تنظیمات Go شرکت را اعمال کنید:

</div>

```bash
go env -w GOPRIVATE=git.basalam.dev
go env -w GOPROXY=https://repo.basalam.dev/artifactory/api/go/basalam-golang-local,https://proxy.golang.org,direct
```

<div dir="rtl">

و اجازه دهید Go با همان کلید SSH که برای git استفاده می‌کنید به GitLab وصل شود:

</div>

```bash
git config --global url."git@git.basalam.dev:".insteadOf "https://git.basalam.dev/"
```

<div dir="rtl">

`GOPRIVATE` باعث می‌شود Go ماژول‌های `git.basalam.dev` را مستقیم از git بگیرد و سراغ پایگاه checksum عمومی نرود. با `go env GOPRIVATE GOPROXY` می‌توانید تنظیمات را بررسی کنید.

</div>

<div dir="rtl">

## شروع سریع

</div>

```bash
gortisan create shop github.com/you/shop   # یا: gortisan create github.com/you/shop
cd shop
cp .env.example .env                       # آدرس PostgreSQL و Redis را تنظیم کنید
./gortisan make:feature user --fields "mobile:string,name:string"
./gortisan make:handler user/list
go run . migrate up
./gortisan run                             # go run . serve، با ری‌استارت بعد از هر تغییر
curl localhost:8000/api/users
```

<div dir="rtl">

`make:feature` پکیج feature را می‌سازد و آن را در `internal/app/serve.go` وصل می‌کند. `make:handler` خود handler، request و response آن و route را می‌نویسد. هر دستور در پایان چاپ می‌کند چه چیزی را باید دستی انجام داد.

سرویسی که **دیتابیسش از قبل وجود دارد** migration را کنار می‌گذارد:

</div>

```bash
gortisan create shop --with postgres,redis,swagger
```

<div dir="rtl">

`--with` از بین `postgres`، `redis`، `migrations` (به postgres نیاز دارد) و `swagger` انتخاب می‌کند و پیش‌فرض همه است. با `gortisan add <component>` می‌شود بعداً اضافه کرد.

## ساختار پروژه

</div>

```
shop/
├── main.go                      cmd.Execute() و اطلاعات کلی Swagger
├── cmd/                         فقط دستورهای Cobra: آرگومان‌ها، بعد internal/app
│   ├── root.go                  همهٔ دستورهای سرویس، در یک فهرست
│   ├── serve.go                 shop serve
│   └── migrate.go               shop migrate up | down | status
├── docs/                        خروجی swag — دستی ویرایش نمی‌شود
├── internal/
│   ├── app/                     wiring: اتصال‌ها ← featureها ← routeها
│   │   ├── serve.go
│   │   └── migrate.go
│   ├── config/                  Config، خوانده‌شده از environment و .env
│   ├── user/                    یک پکیج برای هر feature (make:feature user)
│   │   ├── user.go              entity و خطاها
│   │   ├── service.go           use caseها و interface Repository که لازم دارند
│   │   ├── postgres.go          Repository روی دیتابیس
│   │   ├── handler.go           handlerهای echo با کامنت‌های swag
│   │   ├── dto.go               requestها و responseها
│   │   ├── routes.go            RegisterRoutes(api, h): مسیرها زیر BASE_PATH
│   │   └── service_test.go      table-driven با Repository جعلی
│   ├── sales/order/             featureها را می‌شود گروه‌بندی کرد
│   ├── server/                  echo، middleware/، response/ (RFC 9457)، validate/، validator/
│   ├── platform/                postgres.go، redis.go، orm/، trans/
│   ├── clients/wallet/          کلاینت سرویس‌های داخلی خودمان   (make:client)
│   ├── external/bale/           کلاینت APIهای بیرونی             (make:external)
│   └── migrations/              migrationهای SQL، embed شده (goose)
├── tests/arch_test.go           قواعد وابستگی، با go test ./... اجرا می‌شود
├── deploy/                      Dockerfile و entrypoint.sh
├── AGENTS.md، CLAUDE.md، .agents/skills/   قواعد و skillها برای coding agentها
├── .gortisan.json               ساختار و اجزای پروژه، برای نسخه‌های بعدی gortisan
└── gortisan، gortisan.bat، gortisan.ps1
```

<div dir="rtl">

**جهت وابستگی:** `main.go ← cmd ← internal/app ← feature (handler ← service ← Repository → postgres)`. فقط `internal/app` نوع‌های واقعی را می‌شناسد. featureها از `server`، `platform`، `clients` و `external` استفاده می‌کنند؛ این‌ها هیچ‌وقت featureی را import نمی‌کنند، و هیچ featureی هم feature دیگری را import نمی‌کند — یک interface کوچک تعریف می‌کند و `internal/app` feature دیگر را به آن می‌دهد. اگر قاعده‌ای شکسته شود، `tests/arch_test.go` تست را رد می‌کند.

**مسیرها:** همهٔ featureها زیر `BASE_PATH` (پیش‌فرض `/api`) سوار می‌شوند و `routes.go` هر feature مسیر خودش را نسبت به آن تعیین می‌کند؛ مثلاً `api.Group("/users")` می‌شود `/api/users`. featureی که نسخه لازم دارد، آن را در مسیرش می‌گذارد (`/v2/users`). `/ping`، `/health` و `/swagger/*` بیرون از `BASE_PATH` هستند.

## دستورها

| دستور | کاری که می‌کند |
|---|---|
| `create <name> [<module>]` | پروژهٔ جدید در `./<name>`؛ `--with`، `--no-tidy` |
| `make:feature <name>` | پکیج feature، وصل‌شده در `internal/app`؛ `--fields`، `--table`، `--soft-delete`؛ با migrations، migration ساخت جدولش |
| `make:handler <feature>/<action>` | متد handler (با کامنت swag)، `<Action>Request` و `<Action>Response` در `dto.go`، و route در `routes.go` |
| `make:migration <name>` | فایل خالی `internal/migrations/<time>_<name>.sql` |
| `make:enum <feature>/<name>` | یک enum: ثابت‌ها، `String`، `Valid`، `Parse…`؛ `--members`، `--type` |
| `make:cache <feature>/<name>` | یک cache تایپ‌شدهٔ Redis در `cache.go` همان feature؛ `--type` |
| `make:validator [<group>/]<name>` | ولیدیتور قابل استفادهٔ مجدد در `internal/server/validator` و کلید پیامش در `fa.json` |
| `make:middleware <name>` | `internal/server/middleware/<name>.go` |
| `make:client <name>` / `make:external <name>` | کلاینت HTTP در `internal/clients/<name>` / `internal/external/<name>` |
| `make:test <feature>[/<action>]` | تست سرویس، یا تست HTTP یک action |
| `add <component>` | اضافه کردن postgres، redis، migrations یا swagger به پروژهٔ موجود |
| `run [command] [-- args]` | build و اجرای `serve` (یا دستور دیگر)، با ری‌استارت بعد از هر تغییر |
| `route:list` | همهٔ routeها: متد، مسیر، handler، فایل:خط |
| `swag` | ساخت دوبارهٔ `docs/` با swag پین‌شده — نیازی به نصب نیست |
| `upgrade [version]` | پین کردن نسخهٔ دیگری از gortisan در پروژه |
| `version`، `completion`، `help` | |

`make feature user` (با فاصله) هم کار می‌کند. `--force` فایل‌هایی را که make می‌نویسد کامل بازنویسی می‌کند؛ بدون آن فایل‌های موجود دست نمی‌خورند.

داخل پروژه از اسکریپت `./gortisan` استفاده کنید (`gortisan.bat` یا `.\gortisan.ps1` در ویندوز). این اسکریپت با `go run` همان نسخه‌ای از gortisan را اجرا می‌کند که پروژه پین کرده؛ پس همه کد یکسان تولید می‌کنند و کسی لازم نیست gortisan را نصب کند.

## featureها

</div>

```bash
./gortisan make:feature sales/order --fields "user_id:int64,amount:int64,paid_at:time?" --soft-delete
./gortisan make:handler sales/order/create    # POST /api/sales/orders
./gortisan make:handler sales/order/get       # GET  /api/sales/orders/:id
```

<div dir="rtl">

نام action مسیر را تعیین می‌کند: `list` و `create` روی خود مجموعه، `get`، `update`، `patch` و `delete` روی `/:id`، و هر نام دیگری روی `/<action>` (`mark_paid` → `GET /mark-paid`). در `routes.go` آزادانه تغییرش دهید؛ `make:handler`های بعدی مسیر گروه را از همان‌جا می‌خوانند.

نوع فیلدها: `string`، `int`، `int32`، `int16`، `float`، `bool`، `time`، `json`. علامت `?` در انتها ستون را nullable می‌کند (در Go اشاره‌گر می‌شود).

## migration

</div>

```bash
./gortisan make:migration add_index_to_users   # internal/migrations/<time>_add_index_to_users.sql
go run . migrate up                            # همچنین: down (آخرین)، status
```

<div dir="rtl">

هر migration یک فایل SQL با بخش Up و Down است که [goose](https://github.com/pressly/goose) اجرایش می‌کند:

</div>

```sql
-- +goose Up
CREATE INDEX users_mobile ON users (mobile);

-- +goose Down
DROP INDEX users_mobile;
```

<div dir="rtl">

فایل‌ها داخل باینری embed می‌شوند. یک advisory lock در Postgres باعث می‌شود در هر لحظه فقط یک پروسه migrate کند. در production، اگر `RUN_MIGRATIONS=true` باشد، `deploy/entrypoint.sh` قبل از دستور اصلی `migrate up` را اجرا می‌کند. نسخه‌ها timestamp هستند، پس دو branch هیچ‌وقت شمارهٔ تکراری نمی‌سازند.

## Swagger

هر handlerی که `make:handler` می‌نویسد کامنت‌های swag دارد و `main.go` اطلاعات کلی API را. سند را دوباره بسازید و UI را باز کنید:

</div>

```bash
./gortisan swag              # go run github.com/swaggo/swag/cmd/swag@<pinned> init
open http://localhost:8000/swagger/index.html
```

<div dir="rtl">

سرو کردنش با [echo-swagger](https://github.com/swaggo/echo-swagger) نسخهٔ v2 است که از echo v5 پشتیبانی می‌کند.

## لایهٔ دیتابیس

اتصال به PostgreSQL با [bun](https://bun.uptrace.dev) روی [pgx](https://github.com/jackc/pgx) است. entity یک struct ساده با تگ‌های bun است:

</div>

```go
// internal/user/user.go
type User struct {
	bun.BaseModel `bun:"table:users"`

	ID        int64      `bun:"id,pk,autoincrement" json:"id"`
	Mobile    string     `bun:"mobile" json:"mobile"`
	CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}
```

<div dir="rtl">

`internal/platform/orm` لایه‌ای کوچک روی bun است که داخل پروژهٔ خودتان قرار دارد و فقط bun، pgx و کتابخانهٔ استاندارد را import می‌کند: `Repository[T]` (`Find`، `Get`، `First`، `Count`، `Exists`، `Paginate`، `Create`، `Update`، `Delete`، `ForceDelete`، `Restore`)، scopeها (`Where`، `OrderBy`، `Limit`، `WithTrashed`، `ForUpdate` و …)، relationها (`With("Items.Statuses")`)، صفحه‌بندی و تراکنش صریح:

</div>

```go
err := orm.InTx(ctx, db, func(ctx context.Context, tx orm.IDB) error {
	if err := orders.WithTx(tx).Create(ctx, o); err != nil {
		return err // rollback
	}
	return payments.WithTx(tx).Create(ctx, p) // nil یعنی commit
})
```

<div dir="rtl">

سرویس به interface `Repository` خودش وابسته است، پس `service_test.go` با یک نسخهٔ جعلی و بدون دیتابیس اجرا می‌شود. `postgres.go` همیشه `ErrNotFound` خود feature را برمی‌گرداند، نه خطای orm را.

## اعتبارسنجی و خطاها

هر request بعد از bind خودش را اعتبارسنجی می‌کند و همهٔ فیلدهای نامعتبر را با هم در یک 422 گزارش می‌دهد:

</div>

```go
func (r *CreateRequest) Validate(ctx context.Context) error {
	var errs validate.Errors
	if err := errs.Check(ctx, address.NewMobile("body.mobile", r.Mobile)); err != nil {
		return err
	}
	if r.Amount <= 0 {
		errs.Add("body.amount", "general:min", "مبلغ", 1)
	}
	return errs.Err()
}
```

<div dir="rtl">

`make:validator address/mobile` یک بررسی قابل استفادهٔ مجدد در `internal/server/validator/address/` می‌سازد و شما فقط `Passes` آن را می‌نویسید.

هر خطا — اعتبارسنجی، `echo.NewHTTPError`، route ناموجود یا 500 — را `internal/server` به‌صورت problem details طبق [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) (`application/problem+json`) می‌نویسد. متن‌ها کلیدهای `internal/platform/trans/lang/fa.json` هستند و اگر کد از کلیدی استفاده کند که در فایل نیست، تست رد می‌شود. feature خطاها را با زبان کسب‌وکار برمی‌گرداند و handler status را انتخاب می‌کند.

## دستورهای بیشتر: worker و schedule

هر پروسهٔ جدید یک دستور Cobra جدید است، کنار `serve`:

</div>

```
cmd/worker.go          shop worker    → internal/app/worker.go (consumerها را می‌سازد)
cmd/schedule.go        shop schedule  → internal/app/schedule.go
internal/order/consumer.go            consumer هم مثل handler فقط order.Service را صدا می‌زند
```

<div dir="rtl">

به‌اضافهٔ یک خط در `cmd/root.go`. یک image همهٔ نقش‌ها را اجرا می‌کند: `serve`، `worker` یا `schedule` به‌عنوان دستور.

## نسخه‌ها و به‌روزرسانی

هر پروژه نسخهٔ gortisan را در `./gortisan` پین می‌کند و ساختارش را در `.gortisan.json` ثبت می‌کند. از `v1.0.0` به بعد، هر نسخه از همان major پروژه‌های موجود را سالم نگه می‌دارد و `./gortisan upgrade` پروژه را به جدیدترین نسخه می‌برد. تعهد کامل در [COMPATIBILITY.fa.md](COMPATIBILITY.fa.md) آمده است.

## بدون جادو: قواعد

1. **generator import نمی‌شود.** فقط پکیج‌های خود پروژه و کتابخانه‌هایی که انتخاب کرده‌اید. تست‌های gortisan این را در هر build چک می‌کنند.
2. **وابستگی‌ها پاس داده می‌شوند، نه جست‌وجو.** constructorها هرچه لازم دارند می‌گیرند و `internal/app` آن‌ها را می‌سازد. هیچ چیزی از context یا registry بیرون کشیده نمی‌شود.
3. **featureها از هم جدا می‌مانند.** هیچ featureی feature دیگری را import نمی‌کند و کد مشترک هم هیچ featureی را. `tests/arch_test.go` این را اجبار می‌کند.
4. **کد خوانا به‌جای سازوکار پنهان.** config هر متغیر را با اسمش می‌خواند. دیتابیس از bun استفاده می‌کند و `orm` نازک روی آن داخل ریپوی خودتان است.
5. **خطاها معنا دارند:** feature آن‌ها را نام‌گذاری می‌کند، handler به status تبدیلشان می‌کند و `internal/server` آن‌ها را می‌نویسد.

پروژهٔ تولیدشده به echo v5 و Cobra وابسته است، به‌اضافهٔ هرچه انتخاب کنید: bun و pgx، go-redis، goose، echo-swagger. Kafka، metrics، Sentry یا tracing را خودتان اضافه می‌کنید — یک client در `platform/`، یک middleware، یک خط در `internal/app`.

## توسعه

</div>

```bash
go test ./...          # برای چند ترکیب --with پروژه می‌سازد، همهٔ make:*ها را اجرا می‌کند،
                       # بعد go mod tidy، go vet و go test می‌گیرد (ماژول دانلود می‌کند)،
                       # و همین کار را روی fixtureهای سازگاری انجام می‌دهد
go test -short ./...   # فقط تست‌های واحد
```

<div dir="rtl">

قالب‌ها در `internal/generator/templates` هستند و داخل باینری embed می‌شوند. هر release یک تگ git است: `git tag -a v0.1.0 -m v0.1.0 && git push origin v0.1.0`.

</div>
