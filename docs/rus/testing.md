# Тесты и диагностика производительности

[English](../eng/testing.md) · [Документация](README.md)

Выполняйте команды из корня репозитория. Результаты тестов, профили и временные миры храните отдельно от рабочих данных сервера.

Для полного набора тестов нужны `.mcdata/1.21.4/vanilla/` и `.mcdata/1.21.4/loot_tables/`. Подготовьте их [скриптами генерации данных](generators/README.md). CI скачивает серверный JAR с серверов Mojang, проверяет его контрольную сумму и создаёт эти ресурсы с Java 21 перед запуском тестов.

## Основные проверки

Linux (Bash):

```bash
go build ./...
make test
make lint
make docs-check
```

Windows (PowerShell 7.2+):

```powershell
go build ./...
go test ./... -count=1
go vet ./...
go run ./cmd/vitis-doccheck
```

Для конкурентных изменений запускайте `make test-race`. Обнаружение гонок требует поддерживаемой платформы, инструментов сборки и cgo. `make fuzz-smoke` запускает три краткие проверки чтения данных протокола, NBT и распаковки. `make verify` объединяет тесты, go vet, проверку гонок, фаззинг и проверку документации.

Во время разработки начинайте с затронутых пакетов:

Linux (Bash):

```bash
go test ./internal/protocol/... ./internal/network/... ./internal/session -count=1
go test ./internal/world/... -count=1
go test ./cmd/vitis-doccheck -count=1
```

Windows (PowerShell 7.2+):

```powershell
go test ./internal/protocol/... ./internal/network/... ./internal/session -count=1
go test ./internal/world/... -count=1
go test ./cmd/vitis-doccheck -count=1
```

Проверяйте сообщения о пропущенных тестах. Для части сравнений и измерений генерации нужны ресурсы Minecraft вне Git или эталонный мир. Совместимость с клиентом проверяется отдельно.

## Сценарии тестирования

| Изменение | Что проверить |
| --- | --- |
| Сетевой кодек | Кодирование и декодирование, обрезанные данные, лимиты размера, неверные VarInt/NBT/компоненты и числа |
| Жизненный цикл сессии | Неверный этап, повторное подтверждение, ошибка авторизации, повторный вход, отключение и остановка |
| Очередь мира | Переполнение, отказ задачи, ограничение выполнения за тик, отмена и доступ к изменяемому состоянию |
| Сохранение | Изменение после снимка, ошибка записи, завершение ожидающих записей, выгрузка, повторное открытие и сохранение метаданных |
| Игровые механики | Некорректные запросы, режим игры и права, синхронизация клиента, смерть, повторный вход и сохранённое состояние |
| Ресурсы | Ошибочные метаданные, неподдерживаемые определения, сохранение прежнего набора при ошибке компиляции |
| Документация | Неверная ссылка, отсутствующая страница или ключ, повторный или устаревший ключ; примеры кода не считаются строками справочника |

## Проверка клиентом

Подготовьте ресурсы, создайте временный мир и подключите клиент 1.21.4. Проверьте вход, загрузку чанков, движение, инвентарь, действие с блоком, отключение, повторный вход и остановку с перезапуском. Проверку аккаунтов тестируйте с авторизованным клиентом отдельно от разработки без авторизации.

Поддерживаемые действия перечислены в [статусе](status.md), состав отчёта об ошибке — в [диагностике](troubleshooting.md).

## Измерения и профили

Выбирайте тест производительности для участка кода, который исследуете:

Linux (Bash):

```bash
go test ./internal/network -run='^$' -bench=BenchmarkChunkCompression -benchmem -count=3
go test ./internal/entity -run='^$' -bench=. -benchmem -count=3
```

Windows (PowerShell 7.2+):

```powershell
go test ./internal/network -run='^$' -bench=BenchmarkChunkCompression -benchmem -count=3
go test ./internal/entity -run='^$' -bench=. -benchmem -count=3
```

Небольшая нагрузка генерации мира после подготовки ресурсов:

Linux (Bash):

```bash
VITIS_BENCH_SIDE=3 go test ./internal/world -run='^$' \
  -bench=BenchmarkPregenerationFreshWorld -benchtime=1x -count=1 -benchmem
```

Windows (PowerShell 7.2+):

```powershell
$previousSide = $env:VITIS_BENCH_SIDE
try {
    $env:VITIS_BENCH_SIDE = '3'
    go test ./internal/world -run='^$' -bench=BenchmarkPregenerationFreshWorld -benchtime=1x -count=1 -benchmem
}
finally { $env:VITIS_BENCH_SIDE = $previousSide }
```

Без переменной окружения тест использует значительно большую область. Параметры измерений, таймаутов, трассировки и кеша описаны в [исходнике](../../internal/world/pregeneration_perf_test.go).

Сбор профилей выбранной нагрузки:

Linux (Bash):

```bash
go test ./internal/network -run='^$' -bench=BenchmarkChunkCompression \
  -cpuprofile=/tmp/vitis-cpu.pprof -memprofile=/tmp/vitis-heap.pprof
go tool pprof /tmp/vitis-cpu.pprof
```

Windows (PowerShell 7.2+):

```powershell
$cpuProfile = Join-Path $env:TEMP 'vitis-cpu.pprof'
$heapProfile = Join-Path $env:TEMP 'vitis-heap.pprof'
go test ./internal/network -run='^$' -bench=BenchmarkChunkCompression `
  -cpuprofile=$cpuProfile -memprofile=$heapProfile
go tool pprof $cpuProfile
```

Профили показывают затраты процессора и памяти выбранного теста. Команда может создать исполняемый файл тестов; не добавляйте его в Git.

Для сравнения чанка нужен независимый эталонный мир Minecraft 1.21.4:

Linux (Bash):

```bash
go run ./cmd/vitis-worldgen-compare -version 1.21.4 -world /tmp/vitis-reference-world \
  -dimension overworld -x 0 -z 0 -top 10
```

Windows (PowerShell 7.2+):

```powershell
$referenceWorld = Join-Path $env:TEMP 'vitis-reference-world'
go run ./cmd/vitis-worldgen-compare -version 1.21.4 -world $referenceWorld `
  -dimension overworld -x 0 -z 0 -top 10
```

`-version` выбирает `.mcdata/<version>/vanilla`; `-resources` задаёт другой путь. DataVersion мира и сгенерированные таблицы должны соответствовать поддерживаемой версии сервера.

Записывайте коммит, версию ресурсов, начальное значение генерации, измерения, дальности, число игроков, процессор, память и накопитель. Повторяйте одинаковую нагрузку для сравнения. Метрики списка игроков и прегенерации учитывают разные этапы работы. HTTP-интерфейс профилирования и экспорта метрик сейчас не настроен.

Полная генерация восстанавливает выходные файлы при сбое; см. [руководство](generators/README.md). Для сравнений, читающих `.mcdata`, нужны данные разработчика. Обычный запуск бинарника подготавливает ресурсы внутри мира.

На Windows `pwsh -NoProfile -File .\scripts\windows\verify.ps1` выполняет обязательные проверки без Make. Тесты инструментов запускаются через `go test ./scripts -run 'TestPowerShell|TestGenerationManifest' -count=1`; нужен PowerShell 7.2+, Pester не требуется. Они также выполняются в Windows CI, дополняя проверки на Linux.

Для метрик процесса на Windows используйте `pwsh -NoProfile -File .\scripts\windows\measure_worldgen.ps1 metrics.json <program> <arguments>`. Поля JSON совпадают с Linux-инструментом, но `peak_rss_kib` измеряет максимальный рабочий набор Windows и напрямую не сравним с Linux `Maxrss`. Частота процессора и температура остаются нулевыми. Метрики записываются и при ошибке нагрузки; код выхода сохраняется. Рабочий каталог остаётся прежним.

Соберите исполняемый файл тестов из корня репозитория, затем запустите его из каталога пакета, чтобы относительные пути ресурсов работали:

Linux (Bash):

```bash
go test -c -o /tmp/vitis-worldgen-bench ./internal/world
(
  cd internal/world
  VITIS_BENCH_SIDE=3 go run ../../scripts/linux/measure_worldgen.go /tmp/vitis-metrics.json /tmp/vitis-worldgen-bench \
    '-test.run=^$' -test.bench=BenchmarkPregenerationFreshWorld -test.benchtime=1x -test.count=1 -test.benchmem
)
```

Windows (PowerShell 7.2+):

```powershell
$benchmark = Join-Path $env:TEMP 'vitis-worldgen-bench.exe'
$metrics = Join-Path $env:TEMP 'vitis-metrics.json'
go test -c -o $benchmark ./internal/world
$previousSide = $env:VITIS_BENCH_SIDE
Push-Location internal/world
try {
    $env:VITIS_BENCH_SIDE = '3'
    pwsh -NoProfile -File ..\..\scripts\windows\measure_worldgen.ps1 $metrics $benchmark `
      '-test.run=^$' -test.bench=BenchmarkPregenerationFreshWorld -test.benchtime=1x -test.count=1 -test.benchmem
}
finally {
    $env:VITIS_BENCH_SIDE = $previousSide
    Pop-Location
}
```
