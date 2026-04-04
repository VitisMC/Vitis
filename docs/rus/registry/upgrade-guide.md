# Система регистров — Руководство по обновлению

Это руководство описывает пошагово, как обновить систему регистров Vitis при переходе на новую версию Minecraft (например, с 1.21.4 на 1.22).

> **Необходимые инструменты**: Linux или macOS, установленный Go, `curl` и `python3`, Java 21+ (для генерации `registries.json`).

### Что нужно обновить

При выходе новой версии Minecraft могут измениться три вещи:

1. **ID регистров** — блоки, предметы, сущности и т.д. могут быть добавлены, удалены или перенумерованы
2. **Данные конфигурационных регистров** — эффекты зачарований, свойства биомов, типы урона и т.д.
3. **Теги** — группы тегов (напр. `#minecraft:enchantable/armor`) могут получить или потерять записи

Все три типа данных обрабатываются одним конвейером кодогенерации. Процесс обновления:

```
1. Сгенерировать registries.json из JAR ванильного сервера новой версии
2. Скачать JSON конфигурационных регистров для новой версии
3. Скачать JSON тегов для новой версии
4. Запустить кодогенератор
5. Обновить версионные константы
6. Протестировать
```

### Шаг 1: Генерация `registries.json`

#### Что такое `registries.json`?

Этот файл содержит все 80 маппингов имя↔ID встроенных регистров. Он генерируется самим ванильным сервером Minecraft через встроенный генератор данных.

#### Где хранится

```
.mcdata/1.21.4/registries.json
```

Для новой версии путь: `.mcdata/{версия}/registries.json`, напр. `.mcdata/1.22/registries.json`.

#### Как сгенерировать

1. **Скачайте JAR ванильного сервера** целевой версии с [официального манифеста Minecraft](https://piston-meta.mojang.com/mc/game/version_manifest_v2.json).

   Найти прямую ссылку на скачивание можно так:
   ```bash
   curl -s https://piston-meta.mojang.com/mc/game/version_manifest_v2.json \
     | python3 -c "
   import sys, json
   manifest = json.load(sys.stdin)
   for v in manifest['versions']:
       if v['id'] == '1.22':  # ← заменить на нужную версию
           print(v['url'])
           break
   "
   ```

   Или просто скачайте с https://www.minecraft.net/en-us/download/server.

2. **Запустите генератор данных**:
   ```bash
   mkdir -p /tmp/mc-datagen && cd /tmp/mc-datagen
   java -DbundlerMainClass="net.minecraft.data.Main" -jar server.jar --all
   ```

   Это создаст директорию `generated/`. Нужный файл:
   ```
   generated/reports/registries.json
   ```

3. **Скопируйте в директорию данных проекта**:
   ```bash
   mkdir -p /path/to/vitis/.mcdata/1.22
   cp generated/reports/registries.json \
     /path/to/vitis/.mcdata/1.22/registries.json
   ```

### Шаг 2: Скачивание JSON конфигурационных регистров

#### Что это за файлы?

JSON-файлы, описывающие каждую запись в 12 конфигурационных регистрах (зачарования, биомы, типы урона и т.д.). На этапе сборки они конвертируются в NBT и отправляются клиентам, у которых нет ванильного датапака.

#### Источник

Самый удобный источник — репозиторий [misode/mcmeta](https://github.com/misode/mcmeta) на GitHub. Он публикует извлечённые данные для каждой версии Minecraft как git-теги.

Формат тега: `{версия}-data-json` (напр. `1.21.4-data-json`, `1.22-data-json`).

Данные находятся в `data/minecraft/{имя_регистра}/` в соответствующем теге.

#### Как скачать

1. **Запустите `scripts/update_version.sh`**:

   Передайте новую версию как аргумент. Массив `REGISTRIES` содержит все датапаки для скачивания:
   ```bash
   ./scripts/update_version.sh 1.22
   ```

   Если в новой версии добавились или удалились конфигурационные регистры, обновите массивы `REGISTRIES` и `WORLDGEN_REGISTRIES`:
   ```bash
   REGISTRIES=(
     "banner_pattern"
     "chat_type"
     "damage_type"
     "dimension_type"
     "enchantment"
     "instrument"
     "jukebox_song"
     "painting_variant"
     "trim_material"
     "trim_pattern"
     "wolf_variant"
   )
   ```

   > **Как узнать какие регистры существуют**: Проверьте [страницу протокола на Minecraft Wiki](https://minecraft.wiki/w/Java_Edition_protocol) для вашей версии — в разделе Configuration перечислены все регистры, которые ванильный сервер отправляет через `RegistryData`.

2. Скрипт скачает все JSON-файлы в `.mcdata/{версия}/datapacks/`.

#### Проверка

Убедитесь, что директории регистров содержат `.json` файлы:
```bash
for dir in .mcdata/1.22/datapacks/*/; do
  count=$(ls -1 "$dir"*.json 2>/dev/null | wc -l)
  echo "$(basename $dir): $count записей"
done
```

### Шаг 3: Скачивание JSON тегов

#### Что такое теги?

Теги определяют именованные группы записей регистров. Например, `tags/item/enchantable/armor.json` перечисляет все предметы, на которые можно наложить зачарования брони. Теги критически важны — без них клиент не может разобрать данные зачарований и вылетит.

#### Источник

Тот же репозиторий: [misode/mcmeta](https://github.com/misode/mcmeta), тот же тег (`{версия}-data-json`).

Теги расположены в `data/minecraft/tags/{имя_регистра}/` и могут содержать поддиректории (напр. `tags/item/enchantable/`, `tags/block/mineable/`).

#### Как скачать

Тот же скрипт `scripts/update_version.sh` обрабатывает и теги. Если в новой версии появились новые категории тегов, обновите массив `TAG_REGISTRIES`:

```bash
TAG_REGISTRIES=(
  "banner_pattern"
  "block"
  "cat_variant"
  "damage_type"
  "enchantment"
  "entity_type"
  "fluid"
  "game_event"
  "instrument"
  "item"
  "painting_variant"
  "point_of_interest_type"
)

TAG_WORLDGEN_REGISTRIES=(
  "worldgen/biome"
  "worldgen/flat_level_generator_preset"
  "worldgen/structure"
  "worldgen/world_preset"
)
```

Теги скачиваются в `.mcdata/{версия}/tags/` автоматически.

#### Проверка

```bash
find .mcdata/1.22/tags -name '*.json' | wc -l
```

Для 1.21.4 это 562 файла. В новых версиях может быть больше.

### Шаг 4: Обновление кодогенератора

#### Что делает генератор

Генератор (`internal/registry/generator/main.go`) читает три источника и создаёт три файла Go:

| Вход | Выходной файл | Содержимое |
|---|---|---|
| `registries.json` | `generated/ids.go` | Все маппинги имя↔ID как `[]string` |
| `registries.json` + JSON датапаков | `generated/config_nbt.go` | Предкодированные NBT-байты |
| `registries.json` + JSON тегов | `generated/tags.go` | Маппинги тег→ID как `map[string][]int32` |

#### Обновление путей в генераторе

Генератор читает из `.mcdata/{версия}/` через флаг `-version`:

```bash
go run ./internal/registry/generator/ -version 1.22
```

Менять пути не нужно — генератор использует `.mcdata/{версия}/` автоматически.

#### Обновление списка конфигурационных регистров

Если Minecraft добавил или удалил конфигурационные регистры, обновите map `configRegistries` в генераторе:

```go
var configRegistries = map[string]string{
    "minecraft:banner_pattern":   "banner_pattern",
    "minecraft:chat_type":        "chat_type",
    "minecraft:damage_type":      "damage_type",
    "minecraft:dimension_type":   "dimension_type",
    "minecraft:enchantment":      "enchantment",
    "minecraft:instrument":       "instrument",
    "minecraft:jukebox_song":     "jukebox_song",
    "minecraft:painting_variant": "painting_variant",
    "minecraft:trim_material":    "trim_material",
    "minecraft:trim_pattern":     "trim_pattern",
    "minecraft:wolf_variant":     "wolf_variant",
    "minecraft:worldgen/biome":   "worldgen/biome",
}
```

#### Обновление подсказок типов NBT (при необходимости)

Некоторые JSON-поля требуют явных аннотаций типов NBT, потому что JSON не различает float/double и int/long:

```go
var doubleFields = map[string]bool{
    "coordinate_scale": true,
    "offset":           true,
}

var longFields = map[string]bool{
    "fixed_time": true,
}
```

#### Запуск генератора

```bash
go run ./internal/registry/generator/ -version 1.22
```

### Шаг 5: Обновление `builtin.go`

Если в новой версии Minecraft добавились или удалились встроенные регистры, обновите функцию `builtinIDMap()` в `internal/registry/builtin.go`.

Каждая запись маппит имя регистра на сгенерированную переменную `[]string`:

```go
"minecraft:block": generated.Block,
"minecraft:item":  generated.Item,
```

Имена переменных автоматически генерируются из имён регистров. Соглашение:
- `minecraft:block` → `generated.Block`
- `minecraft:entity_type` → `generated.EntityType`
- `minecraft:worldgen/biome_source` → `generated.WorldgenBiomeSource`

Проверьте начало `generated/ids.go` после перегенерации, чтобы найти точные имена переменных.

### Шаг 6: Обновление версионных констант

#### Версия Known Packs

В `internal/session/configuration_handler.go` обновите строку версии Known Packs:

```go
knownPacks := &cfgpacket.ClientboundKnownPacks{
    Packs: []cfgpacket.KnownPack{
        {Namespace: "minecraft", ID: "core", Version: "1.22"},  // ← обновить
    },
}
```

#### Версия протокола

Обновите номер версии протокола везде, где он определён в проекте. Проверьте [список версий протокола на Minecraft Wiki](https://minecraft.wiki/w/Java_Edition_protocol_version) для правильного номера.

### Шаг 7: Сборка и тестирование

#### Компиляция

```bash
go build ./...
```

Исправьте ошибки компиляции. Типичные проблемы:
- Для новых регистров в `registries.json` нужны соответствующие записи в `builtin.go`
- Удалённые регистры нужно удалить из `builtin.go`

#### Запуск тестов

```bash
go test ./internal/registry/ -v -count=1
```

#### Подключение клиента

Запустите сервер и подключитесь ванильным клиентом Minecraft нужной версии:

```bash
go run ./cmd/vitis -config configs/vitis.yaml
```

Проверьте логи сервера:
```
registry manager: 92 registries, 12 config registries
session X: serverbound_known_packs count=1 vanilla=true
session X: sent 12 registry_data packets (knownVanilla=true)
session X: sent update_tags
session X: sent finish_configuration
session X: acknowledge_finish_configuration, entering play
```

Если клиент отключается с ошибками регистров, проверьте:
1. Все ли записи конфигурационных регистров присутствуют и корректно закодированы в NBT?
2. Отправляются ли все необходимые теги (особенно теги предметов вроде `enchantable/*`)?
3. Корректны ли подсказки типов NBT (`doubleFields`, `longFields`) для новых/изменённых полей?

### Краткая справка: источники данных

| Данные | Источник | Как получить |
|---|---|---|
| `registries.json` | JAR ванильного сервера Minecraft | `java -DbundlerMainClass="net.minecraft.data.Main" -jar server.jar --all` → `generated/reports/registries.json` |
| JSON конфиг-регистров | [misode/mcmeta](https://github.com/misode/mcmeta) тег `{версия}-data-json` | `scripts/update_version.sh` или `data/minecraft/{регистр}/` |
| JSON тегов | [misode/mcmeta](https://github.com/misode/mcmeta) тег `{версия}-data-json` | `scripts/update_version.sh` или `data/minecraft/tags/{регистр}/` |

Альтернативные источники:
- [PrismarineJS/minecraft-data](https://github.com/PrismarineJS/minecraft-data/tree/master/data/pc/) — поддерживается сообществом, содержит данные для многих версий
- Запуск генератора данных ванильного сервера (так же, как для `registries.json`) — директория `generated/data/minecraft/` содержит и записи датапаков, и теги

### Устранение неполадок

#### Клиент вылетает с "Failed to load registries"

Клиент не смог разобрать записи какого-то регистра. Частые причины:
- Отсутствующие или некорректные NBT-данные в записи конфигурационного регистра
- Неверный тип NBT для числового поля (напр. отправка `Float` вместо `Double`)

**Решение**: Проверьте `doubleFields` / `longFields` в генераторе. Сравните NBT-вывод с захватом от ванильного сервера.

#### Клиент вылетает с "Failed to parse local data"

Клиент использует локальные (встроенные) данные (режим Known Packs, `HasData=false`), но не может разрешить некоторые ссылки. Это означает, что **отсутствуют теги**.

**Решение**: Убедитесь, что все регистры тегов включены в `TAG_REGISTRIES` / `TAG_WORLDGEN_REGISTRIES` в скрипте, и что генератор создаёт данные тегов для них.

#### Количество регистров "X registries" не совпадает с ожидаемым

Общее количество — это встроенные (80) + регистры только из конфигурации. Некоторые конфигурационные регистры (например, `minecraft:enchantment`) ТАКЖЕ присутствуют в `registries.json` как встроенные, поэтому они пересекаются. `Manager` дедуплицирует их.

#### Генератор падает с "read datapack dir"

JSON-файлы для конфигурационного регистра отсутствуют. Перезапустите `scripts/update_version.sh` и проверьте, что файлы скачались.

#### Генератор падает с "parse JSON"

Скачанный JSON-файл некорректен (возможно, это ответ GitHub API вместо реальных данных). Удалите директорию `.mcdata/` и перезапустите `scripts/update_version.sh`. При использовании GitHub API имейте в виду лимиты запросов — рассмотрите использование GitHub-токена.
