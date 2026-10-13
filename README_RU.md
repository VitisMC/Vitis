# Vitis

[English](README.md) · [Документация](docs/rus/README.md) · [Участие в разработке](CONTRIBUTING.md)

Серверное ПО для Minecraft Java Edition, написанное на Go.

**Проект находится в экспериментальной стадии.** Игровые механики и совместимость с оригинальным Minecraft реализованы не полностью. Перед использованием ценного мира прочитайте [возможности и ограничения](docs/rus/status.md).

| Совместимость | Версия |
| --- | --- |
| Клиент | Minecraft Java Edition **1.21.4** |
| Протокол | **769** |
| DataVersion мира | **4189** |
| Сборка из исходников | Go **1.23+** |
| Лицензия | [GPLv3](LICENSE) |

## Начало работы

Скачайте архив для своей ОС и процессора из [GitHub Releases](https://github.com/VitisMC/Vitis/releases), распакуйте в каталог сервера и выполните в нём команду.

Linux/macOS:

```bash
./vitis
```

Windows PowerShell:

```powershell
.\vitis.exe
```

При первом запуске Vitis создаёт отсутствующие настройки и списки доступа, скачивает и проверяет официальный `server.jar` Minecraft 1.21.4, затем извлекает ресурсы. Go, Java и исходники не нужны. Существующие файлы сохраняются; при повторном запуске используется локальный пакет.

Подключайтесь клиентом Java Edition 1.21.4. Для сохранения и завершения введите `stop` в консоли. Выбор платформы, файлы первого запуска и ошибки описаны в [установке](docs/rus/getting-started.md); сборка из исходников — в [окружении разработчика](docs/rus/development.md).

Встроенный пакет хранится в `<world>/vanilla/1.21.4/`, пользовательские датапаки — в `<world>/datapacks/`. Каталог мира задаётся в конфигурации.

## Документация

- **Запуск сервера:** [установка](docs/rus/getting-started.md), [конфигурация](docs/rus/configuration.md), [команды](docs/rus/commands.md), [эксплуатация](docs/rus/operations.md), [диагностика](docs/rus/troubleshooting.md).
- **Разработка ядра:** [окружение](docs/rus/development.md), [архитектура](docs/rus/architecture.md), [протокол](docs/rus/protocol.md), [устройство мира](docs/rus/world.md), [добавление функциональности](docs/rus/extending.md).
- **Состояние проекта:** [возможности и ограничения](docs/rus/status.md), [направления развития](docs/rus/roadmap.md).
- **Данные Minecraft:** [ресурсы и генераторы](docs/rus/generators/README.md), [обновление версии](docs/rus/generators/version-upgrade.md).

Плагины Bukkit/Paper и моды Forge/Fabric не поддерживаются.
