# Генераторы данных Vitis

Система кодогенерации, преобразующая файлы данных Minecraft в исходный код Go для совместимости с протоколом.

## Обзор

- **Типобезопасность** — проверка ID и констант на этапе компиляции
- **Производительность** — поиск без аллокаций через сгенерированные карты
- **Поддерживаемость** — единый источник истины для каждого типа данных

Все генераторы расположены в `internal/data/generator/<имя>/main.go`, результат — в `internal/data/generated/<имя>/`.

## Быстрый старт

```bash
# Обновить всё для конкретной версии (рекомендуется)
./scripts/update_version.sh 1.21.4

# Или запустить конкретный генератор
go run internal/data/generator/block/main.go -version 1.21.4
```

## Генераторы (59 штук)

### Из JSON-файлов PrismarineJS

| Генератор | Вход | Описание |
|-----------|------|----------|
| `block` | `blocks.json` | Типы блоков, состояния, свойства (1095 блоков, 27866 состояний) |
| `item` | `items.json` | Типы предметов, размеры стеков (1385 предметов) |
| `entity` | `entities.json` | Типы сущностей, размеры (149 сущностей) |
| `sound` | `sounds.json` | ID звуковых событий (1651 звук) |
| `particle` | `particles.json` | ID типов частиц (112 частиц) |
| `effect` | `effects.json` | Определения эффектов зелий (39 эффектов) |
| `biome` | `biomes.json` | Определения биомов (65 биомов) |
| `spawn_egg` | `items.json` | Яйца призыва (81 яйцо) |
| `packet` | `protocol.json` | Packet ID для всех состояний протокола |
| `recipes` | `recipes.json` | Рецепты крафта (1557 рецептов) |
| `translation` | `language.json` | Ключи переводов (7000 переводов) |
| `collision_shapes` | `blockCollisionShapes.json` + `blocks.json` | AABB коллизий блоков (4989 фигур) |

### Из `registries.json` (генератор данных серверного JAR)

| Генератор | Ключ регистра | Описание |
|-----------|--------------|----------|
| `attribute` | `minecraft:attribute` | Типы атрибутов (31) |
| `game_event` | `minecraft:game_event` | Игровые события (60) |
| `fluid` | `minecraft:fluid` | Типы жидкостей (5) |
| `potion` | `minecraft:potion` | Типы зелий (46) |
| `screen` | `minecraft:menu` | Типы экранов/меню |
| `data_component` | `minecraft:data_component_type` | Типы компонентов данных |
| `block_entity` | `minecraft:block_entity_type` | Типы блок-сущностей |
| `villager_profession` | `minecraft:villager_profession` | Профессии жителей |
| `villager_type` | `minecraft:villager_type` | Типы жителей |
| `cat_variant` | `minecraft:cat_variant` | Варианты кошек |
| `frog_variant` | `minecraft:frog_variant` | Варианты лягушек |
| `message_type` | `minecraft:message_type` | Типы сообщений |
| `registry` | все регистры | Полная система регистров (80 регистров + конфиг NBT + теги) |
| `tag` | все теги | Маппинги тег→ID для 13 регистров |

### Из датапаков misode/mcmeta

| Генератор | Директория | Описание |
|-----------|-----------|----------|
| `damage_type` | `datapacks/damage_type` | Определения типов урона (49) |
| `dimension` | `datapacks/dimension_type` | Определения типов измерений (4) |
| `enchantment` | `datapacks/enchantment` | Определения зачарований (42) |
| `jukebox_song` | `datapacks/jukebox_song` | Песни музыкального автомата (19) |
| `painting_variant` | `datapacks/painting_variant` | Варианты картин (50) |
| `wolf_variant` | `datapacks/wolf_variant` | Варианты волков (9) |
| `instrument` | `datapacks/instrument` | Инструменты (8) |
| `trim_pattern` | `datapacks/trim_pattern` | Паттерны отделки брони (18) |
| `trim_material` | `datapacks/trim_material` | Материалы отделки брони (11) |
| `banner_pattern` | `datapacks/banner_pattern` | Паттерны знамён (43) |
| `structures` | `datapacks/worldgen/structure` | Определения структур |
| `noise_parameter` | `datapacks/worldgen/noise` | Параметры шума |
| `configured_features` | `datapacks/worldgen/configured_feature` | Настроенные объекты |
| `placed_features` | `datapacks/worldgen/placed_feature` | Размещённые объекты |
| `chunk_gen_settings` | `datapacks/worldgen/noise_settings` | Настройки генерации чанков |

### Из декомпилированного серверного JAR

| Генератор | Источник | Описание |
|-----------|---------|----------|
| `chunk_status` | `ChunkStatus.java` | Статусы чанков (12) |
| `entity_pose` | `Pose.java` | Позы сущностей (18) |
| `entity_status` | `EntityEvent.java` | Статусные события сущностей (59) |
| `meta_data_type` | `EntityDataSerializers.java` | Типы сериализаторов метаданных (31) |
| `sound_category` | `SoundSource.java` | Категории звуков (10) |
| `world_event` | `LevelEvent.java` | Мировые события (82) |
| `scoreboard_slot` | `DisplaySlot.java` | Слоты отображения таблицы (19) |
| `game_rules` | `GameRules.java` | Игровые правила (53) |
| `tracked_data` | `Entity*.java` | Отслеживаемые данные сущностей (212) |
| `composter` | `ComposterBlock.java` | Компостируемые предметы (108) |
| `flower_pot` | `FlowerPotBlock.java` | Блоки цветочных горшков (2) |
| `fuels` | `AbstractFurnaceBlockEntity.java` | Топливо для печи (41) |
| `smelting` | `*Recipe.java` | Рецепты готовки (113) |
| `status_effect` | `MobEffect.java` | Статусные эффекты (23) |
| `potion_brewing` | `PotionBrewing.java` | Рецепты зельеварения (41) |

### Генераторы заглушек/ремапов

| Генератор | Описание |
|-----------|----------|
| `block_state_remap` | Ремап ID состояний блоков (миграция версий) |
| `entity_id_remap` | Ремап ID сущностей (миграция версий) |
| `item_id_remap` | Ремап ID предметов (миграция версий) |
| `recipe_remainder` | Остатки рецептов |

## Документация

- [Руководство по обновлению версии](version-upgrade.md) — Обновление до новой версии Minecraft
- [Источники данных](data-sources.md) — Откуда берутся файлы данных

## Структура директорий

```
.mcdata/
└── 1.21.4/
    ├── registries.json          # Сгенерирован из серверного JAR (все 80 регистров)
    ├── blocks.json              # PrismarineJS
    ├── items.json               # PrismarineJS
    ├── entities.json            # PrismarineJS
    ├── protocol.json            # PrismarineJS
    ├── sounds.json              # PrismarineJS
    ├── particles.json           # PrismarineJS
    ├── effects.json             # PrismarineJS
    ├── biomes.json              # PrismarineJS
    ├── language.json            # PrismarineJS
    ├── blockCollisionShapes.json # PrismarineJS
    ├── datapacks/               # misode/mcmeta (данные конфиг-регистров)
    │   ├── damage_type/
    │   ├── dimension_type/
    │   ├── enchantment/
    │   ├── worldgen/biome/
    │   ├── worldgen/structure/
    │   ├── worldgen/noise/
    │   └── ...
    └── tags/                    # misode/mcmeta (определения тегов)
        ├── block/
        ├── item/
        ├── entity_type/
        └── ...

internal/data/
├── generator/                   # Кодогенераторы (59)
│   ├── block/main.go
│   ├── item/main.go
│   └── ...
└── generated/                   # Сгенерированный Go-код (НЕ РЕДАКТИРОВАТЬ)
    ├── block/blocks.go
    ├── item/items.go
    └── ...
```
