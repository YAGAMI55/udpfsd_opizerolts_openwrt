# udpfsd для OpenWrt на Orange Pi Zero

https://img.shields.io/github/v/release/YAGAMI55/udpfsd_opizerolts_openwrt
https://github.com/YAGAMI55/udpfsd_opizerolts_openwrt/actions/workflows/build.yml/badge.svg

Готовый IPK-пакет сервера UDPFS для Orange Pi Zero (ARMv7) под OpenWrt.
Позволяет запускать игры с PlayStation 2 по локальной сети через протокол UDPFS.

---

## 🚀 Особенности

- ✅ Готовый .ipk для установки через opkg
- ✅ Автоматический запуск при загрузке (init-скрипт)
- ✅ Сборка через GitHub Actions (не нужно ставить SDK локально)
- ✅ Поддержка сжатых образов (ZSO, CSO)
- ✅ Работает с Neutrino + NHDDL и OPL
- ✅ Настроен под Orange Pi Zero (ARMv7 Cortex-A7)

---

## 📦 Установка

1. Скачайте IPK из раздела Releases.
   Файл: udpfsd_1.0.0-1_arm_cortex-a7_neon-vfpv4.ipk

2. Загрузите на Orange Pi (через SCP или USB-флешку).

3. Установите командой:
   bash  
   opkg update  
   opkg install /путь/к/udpfsd_*.ipk  
   
   После установки сервис автоматически запустится и добавится в автозагрузку.

---

## ⚙️ Настройка

### Изменить путь к играм

По умолчанию сервер ищет игры в папке /mnt/usb.
Если ваша флешка смонтирована в другом месте — отредактируйте файл:
bash  
vi /etc/init.d/udpfsd  

Измените строку:
bash  
GAMES_DIR="/mnt/usb"  

Затем перезапустите сервис:
bash  
/etc/init.d/udpfsd restart  


### Изменить IP-адрес сервера (если нужно)

Сервер слушает на всех интерфейсах (0.0.0.0).
В клиентских настройках (Neutrino/NHDDL) укажите реальный IP вашего сервера, по-умолчанию он: 192.168.1.10
## yaml  
# nhddl.yaml  
mode: udpfs  
udpfs_ip: 192.168.1.1   # IP вашей PlayStation 2  


---

## 🎮 Использование с PlayStation 2

1. Настройте сеть на PS2 – убедитесь, что PS2 и Orange Pi в одной подсети..

2. Положите образы игр (ISO, CHD, ZSO, CSO) в /mnt/usb/ (или в подпапки, не глубже 5 уровней) поддерживается структура OPL CD/DVD

3. Запустите Neutrino + NHDDL:
   - Скачайте Neutrino и NHDDL.
   - Поместите .elf файлы на карту памяти PS2.
   - В конфиге NHDDL (nhddl.yaml) укажите:  
     mode: udpfs  
     udpfs_ip: 192.168.1.1   # IP вашей PS2  
     
   - Запустите nhddl.elf – игры отобразятся в списке.

---

## 🔧 Управление сервисом

bash  
# Запустить  
/etc/init.d/udpfsd start  
  
# Остановить  
/etc/init.d/udpfsd stop  
  
# Перезапустить  
/etc/init.d/udpfsd restart  
  
# Проверить, запущен ли  
pidof udpfsd  
  
# Посмотреть логи  
logread | grep udpfsd  


---

## 🛠️ Сборка из исходников (для разработчиков)

Если вы хотите собрать IPK самостоятельно:

1. Форкните репозиторий.
2. Внесите изменения.
3. Запустите GitHub Actions вручную или при пуше.
4. Скачайте собранный IPK из артефактов.

Либо соберите локально с помощью OpenWrt SDK – подробности в .github/workflows/build.yml.

---

## 🐛 Устранение неполадок

- Сервис не запускается – проверьте логи: logread | grep udpfsd.
- Игры не видны в NHDDL – проверьте IP в nhddl.yaml и доступность Orange Pi по сети (ping).
- Чёрный экран при запуске – убедитесь, что сервер запущен без флага -ro (в нашем пакете флаг убран) и что есть права на запись (папка nhddl создаётся автоматически).
- Не хватает прав на USB – проверьте монтирование: mount | grep /mnt/usb.

---

## 📜 Лицензия

Исходный код основан на udpfsd и распространяется под той же лицензией (MIT).

---

## 🙏 Благодарности

- pcm720 – за оригинальный udpfsd и NHDDL
- rickgaiser – за Neutrino
- Сообществу PS2-энтузиастов за тестирование и идеи

---

Удачных запусков! 🎮🚀
