# Команды и права

[English](../eng/commands.md) · [Документация](README.md)

В игре вводите команды с `/`, в консоли и RCON — без него. Аргумент в угловых скобках обязателен, в квадратных — необязателен. Справка и автодополнение учитывают права игрока. Для первого оператора подключите игрока и выполните `op PlayerName` в консоли.

Уровень в таблице — минимальный уровень оператора. Консоль и RCON имеют административные права. Команды, которым нужны позиция или инвентарь игрока, доступны только из игры.

## Каталог команд

| Синтаксис | Уровень | Псевдонимы | Примечание |
| --- | --- | --- | --- |
| `/ban <player> [reason]` | 3 | — | Для подключённого игрока используется его UUID, для отсутствующего — UUID по имени |
| `/ban-ip <ip> [reason]` | 3 | — | — |
| `/banlist` | 3 | — | — |
| `/bossbar <add\|remove\|set\|list\|get> ...` | 2 | — | Не подключена к игровой подсистеме |
| `/chunky <world\|shape\|center\|radius\|selection\|start\|progress\|pause\|continue\|cancel>` | 4 | — | Выбор области и управление; см. прегенерацию |
| `/clear [player] [item] [maxCount]` | 2 | — | — |
| `/defaultgamemode <mode>` | 2 | — | Меняет значение в обработчике команд; не сохраняет его и не меняет начальный режим входа |
| `/deop <player>` | 3 | — | — |
| `/difficulty <peaceful\|easy\|normal\|hard>` | 2 | — | — |
| `/effect <give\|clear> <player> [effect] [seconds] [amplifier]` | 2 | — | — |
| `/enchant <player> <enchantment> [level]` | 2 | — | Зачаровывает предмет в руке; интерфейс стола зачарований недоступен |
| `/fill <x1> <y1> <z1> <x2> <y2> <z2> <block>` | 2 | — | — |
| `/gamemode <mode> [player]` | 2 | gm | — |
| `/gamerule <rule> [value]` | 2 | — | — |
| `/give <player> <item> [count]` | 2 | — | — |
| `/help [command]` | 0 | ? | — |
| `/kick <player> [reason]` | 3 | — | — |
| `/kill [player]` | 2 | — | — |
| `/list` | 0 | — | — |
| `/me <action>` | 0 | — | — |
| `/msg <player> <message>` | 0 | tell, w | — |
| `/op <player>` | 3 | — | — |
| `/pardon <player>` | 3 | — | — |
| `/pardon-ip <ip>` | 3 | — | — |
| `/reload` | 2 | — | Перезагрузка датапаков |
| `/say <message>` | 2 | — | — |
| `/scoreboard <objectives\|players> ...` | 2 | — | Не подключена к игровой подсистеме |
| `/seed` | 2 | — | — |
| `/setblock <x> <y> <z> <block>` | 2 | — | — |
| `/setworldspawn [x] [y] [z]` | 2 | — | Обновляет координаты в обработчике и на клиентах; не сохраняет точку появления мира |
| `/spawnpoint [player] [x] [y] [z]` | 2 | — | Отправляет позицию клиенту; не сохраняет точку возрождения игрока |
| `/speed <blocks/second\|reset>` | 2 | — | Только из игры; >0 и ≤200 блоков/с либо reset |
| `/stop` | 4 | — | — |
| `/summon <entity> [x y z]` | 2 | — | — |
| `/team <add\|remove\|join\|leave\|list> ...` | 2 | — | Не подключена к игровой подсистеме |
| `/tellraw <player> <message>` | 2 | — | — |
| `/time set <value> \| /time add <value> \| /time query` | 2 | — | — |
| `/title <player> <title\|subtitle\|actionbar\|clear\|reset\|times> ...` | 2 | — | — |
| `/tp <player> \| /tp <x> <y> <z> \| /tp <player> <x> <y> <z>` | 2 | teleport | — |
| `/weather <clear\|rain\|thunder> [duration]` | 2 | — | — |
| `/whitelist <on\|off\|add\|remove\|list\|reload>` | 3 | — | Включение не сохраняется; добавление по имени вычисляет UUID без проверки аккаунта |
| `/worldborder <center\|set\|add\|warning\|get> ...` | 2 | — | Не подключена к игровой подсистеме |
| `/xp <add\|set> <player> <amount> [points\|levels] \| /xp query <player>` | 2 | experience | — |

## Ограничения

- Используйте явно указанные имена и координаты. Полная поддержка селекторов Minecraft, SNBT и API командных блоков отсутствует.
- `speed` задаёт скорость в блоках в секунду. Шкала отличается от Bukkit; `reset` восстанавливает обычную скорость.
- `chunky` описана в [прегенерации](pregeneration.md).
- После перезапуска повторяйте `whitelist on`. Добавление по имени использует UUID без проверки аккаунта; при включённой авторизации он отличается от UUID Mojang. Блокировка подключённого игрока использует его фактический UUID.
- Изменения правил игры, режима по умолчанию и точек появления через команды сохраняются не полностью. Возрождение у кровати обрабатывается отдельно в жизненном цикле игрока.

Аргументы и подкоманды проверяют [основные](../../internal/command/builtin.go), [игровые](../../internal/command/gameplay_cmds.go) и [многопользовательские команды](../../internal/command/multiplayer_cmds.go), [блокировки](../../internal/command/ban_cmds.go) и [белый список](../../internal/command/whitelist_cmds.go). Состояние изменяют [игровые обработчики команд](../../internal/session/command_bridge.go). См. также [возможности](status.md).
