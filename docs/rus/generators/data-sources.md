# Источники данных

Внешние источники данных, используемые генераторами Vitis. Все данные хранятся в `.mcdata/{версия}/`.

## Основные источники

### 1. Серверный JAR Minecraft — `registries.json`

**Источник:** Официальный серверный JAR от Mojang, встроенный генератор данных.

**Выдаёт:** `registries.json` — все 80 маппингов имя↔ID встроенных регистров с протокольными ID.

**Формат:**
```json
{
  "minecraft:block": {
    "protocol_id": 3,
    "entries": {
      "minecraft:air": { "protocol_id": 0 },
      "minecraft:stone": { "protocol_id": 1 }
    }
  }
}
```

**Как работает в `update_version.sh`:**
1. Серверный JAR скачивается скриптом `extract_loot_tables.sh` (кэшируется в `.mc-decompiled/downloads/{версия}/server.jar`)
2. Запускается генератор данных: `java -DbundlerMainClass="net.minecraft.data.Main" -jar server.jar --reports`
3. Результат `generated/reports/registries.json` копируется в `.mcdata/{версия}/registries.json`

**Используется:** 14 генераторами (attribute, fluid, potion, game_event, screen, data_component, block_entity, villager_profession, villager_type, cat_variant, frog_variant, message_type, registry, tag).

### 2. PrismarineJS/minecraft-data

**Репозиторий:** https://github.com/PrismarineJS/minecraft-data

**Скачиваемые файлы:**
- `blocks.json` — Определения блоков, состояния, свойства
- `items.json` — Определения предметов, размеры стеков
- `entities.json` — Типы сущностей, размеры
- `protocol.json` — Определения пакетов для всех состояний
- `sounds.json` — Имена и ID звуковых событий
- `particles.json` — ID типов частиц
- `effects.json` — Определения эффектов зелий
- `biomes.json` — Определения биомов
- `foods.json` — Свойства еды
- `recipes.json` — Рецепты крафта
- `language.json` — Ключи переводов
- `blockCollisionShapes.json` — Формы коллизий блоков
- `materials.json`, `tints.json`, `enchantments.json`

**Шаблон URL:** `https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/{версия}/{файл}.json`

**Версии:** от 1.8 до последней. Обновляется в течение дней после релиза.

### 3. misode/mcmeta — датапаки и теги

**Репозиторий:** https://github.com/misode/mcmeta

**Используется для:**
- **Датапаки конфигурационных регистров** — JSON-определения для регистров, отправляемых клиентам на этапе Configuration
- **Теги** — Именованные группы записей регистров (теги блоков, предметов и т.д.)

**Тег:** `{версия}-data-json` (напр. `1.21.4-data-json`)

**Скачиваемые датапаки** (в `.mcdata/{версия}/datapacks/`):
- `banner_pattern/`, `chat_type/`, `damage_type/`, `dimension_type/`
- `enchantment/`, `instrument/`, `jukebox_song/`, `painting_variant/`
- `trim_material/`, `trim_pattern/`, `wolf_variant/`
- `worldgen/biome/`, `worldgen/structure/`, `worldgen/noise/`
- `worldgen/configured_feature/`, `worldgen/placed_feature/`, `worldgen/noise_settings/`

**Скачиваемые теги** (в `.mcdata/{версия}/tags/`):
- `block/`, `entity_type/`, `fluid/`, `item/`

**Версии:** от 1.14 до последней. Обновляется в течение часов после релиза.

### 4. Серверный JAR Minecraft — декомпилированный исходный код

**Инструмент:** MaxPixelStudios/MinecraftDecompiler (https://github.com/MaxPixelStudios/MinecraftDecompiler)

**Используется:** 15 генераторами, которые парсят декомпилированный Java-код для извлечения констант и маппингов, недоступных в публичных JSON-данных (позы сущностей, игровые правила, мировые события, отслеживаемые данные и т.д.).

**Процесс:** `update_version.sh` Шаг 5 декомпилирует серверный JAR с помощью Vineflower. Генераторы из массива `GENERATORS` читают из `.mc-decompiled/{версия}-decompiled/`.

## Запасные источники

- **Официальная Minecraft Wiki** — https://minecraft.wiki
- **Articdive/ArticData** — https://github.com/Articdive/ArticData

## Доступность по версиям

| Источник | Мин. версия | Частота обновления |
|----------|-------------|-------------------|
| PrismarineJS | 1.7.10 | Дни после релиза |
| misode/mcmeta | 1.14 | Часы после релиза |
| Генератор данных серверного JAR | Любая | Немедленно (от Mojang) |
| Декомпилированный JAR | Любая | Немедленно (требуется Java) |
