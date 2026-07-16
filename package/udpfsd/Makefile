include $(TOPDIR)/rules.mk

PKG_NAME:=udpfsd
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)

include $(INCLUDE_DIR)/package.mk

define Package/$(PKG_NAME)
  SECTION:=net
  CATEGORY:=Network
  TITLE:=UDPFS/UDPBD server for PlayStation 2 (Go)
  URL:=https://github.com/YAGAMI55/udpfsd_opizerolts_openwrt
  DEPENDS:=   # пусто – бинарник статический
endef

define Package/$(PKG_NAME)/description
  A Go implementation of UDPFS/UDPRDMA server for PS2.
endef

# Копируем готовый бинарник (собранный заранее)
define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	$(CP) ./src/udpfsd $(PKG_BUILD_DIR)/
endef

define Build/Configure
	# пусто
endef

define Build/Compile
	# пусто
endef

define Package/$(PKG_NAME)/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/udpfsd $(1)/usr/bin/

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/udpfsd.init $(1)/etc/init.d/udpfsd
endef

define Package/$(PKG_NAME)/postinst
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ] && [ -x /etc/init.d/udpfsd ]; then
	/etc/init.d/udpfsd enable
	/etc/init.d/udpfsd start
fi
endef

$(eval $(call BuildPackage,$(PKG_NAME)))
