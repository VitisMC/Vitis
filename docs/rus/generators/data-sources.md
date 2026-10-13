# Источники данных Minecraft

[English](../../eng/generators/data-sources.md) · [Документация](../README.md)

## Файлы и источники

| Файл | Источник / назначение |
| --- | --- |
| `mojang-version.json` | Метаданные версии из официального списка Mojang |
| `.mc-decompiled/downloads/<version>/server.jar` | Официальный JAR сервера; SHA-1 проверяется по метаданным |
| `server_mappings.txt` | Таблицы имён Mojang с проверкой SHA-1 |
| `<world>/vanilla/<version>/` | Ресурсы сервера, описание пакета и версия |
| `.mcdata/<version>/vanilla` | Результат генератора данных, встроенные ресурсы и отчёты; источник переноса в мир |
| `registries.json`, `datapacks/`, `tags/`, `recipes/`, `loot_tables/` | Копии ресурсов официального пакета для генераторов кода |
| JSON выбранной версии PrismarineJS | Блоки, предметы, сущности, протокол, биомы, формы блоков и связанные данные |
| `source.lock` | Закреплённый коммит minecraft-data и хеши официальных JAR и таблиц имён |
| `checksums.sha256` | Контрольные суммы подготовленных файлов |
| `.mc-decompiled/<version>-decompiled` | Java-исходники с читаемыми именами для генераторов |
| `.mc-decompiled/<version>-remapped.jar` | JAR с переименованными классами для Java-экспортёров |

Полная генерация использует ресурсы из официального JAR и закреплённую версию [PrismarineJS minecraft-data](https://github.com/PrismarineJS/minecraft-data). Снимки misode в текущем процессе не используются.

## Воспроизводимость

JAR, таблицы имён, ресурсы, исходники и JSON протокола должны относиться к одной версии Minecraft. Проверяйте хеши из метаданных Mojang, версию ресурсов, коммит источника и изменения сгенерированных файлов.

`vitis-data` проверяет встроенные сведения о версии и содержимое пакета. Скачивание JAR и проверку его опубликованного хеша выполняют подготовка ресурсов сервера и скрипт полной генерации.

MinecraftDecompiler и Vineflower готовят исходники и переименованный JAR. Версии инструментов, сведения о входных файлах и контрольные суммы результата записываются в `.mc-decompiled/<version>.sources.lock` и `.sources.sha256`. Требования Java-экспортёров описаны в [руководстве](../../../internal/data/generator/block/worldgen/README.md).

## Исходники

[Подготовка на Linux](../../../scripts/linux/update_version.sh), [подготовка на Windows](../../../scripts/windows/update_version.ps1), [извлечение ресурсов](../../../internal/data/vanilla/prepare.go), [генерация отчётов](../../../internal/data/vanilla/generate.go), [команда](../../../cmd/vitis-data/main.go), [генераторы](../../../internal/data/generator/).

Перенос ядра описан в [обновлении версии](version-upgrade.md).
