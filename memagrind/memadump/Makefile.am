include $(top_srcdir)/Makefile.tool.am

EXTRA_DIST = docs/nl-manual.xml

#----------------------------------------------------------------------------
# memadump-<platform>
#----------------------------------------------------------------------------

noinst_PROGRAMS  = memadump-@VGCONF_ARCH_PRI@-@VGCONF_OS@
if VGCONF_HAVE_PLATFORM_SEC
noinst_PROGRAMS += memadump-@VGCONF_ARCH_SEC@-@VGCONF_OS@
endif

NONE_SOURCES_COMMON = md_main.c lz4.c

memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_SOURCES      = \
	$(NONE_SOURCES_COMMON)
memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_CPPFLAGS     = \
	$(AM_CPPFLAGS_@VGCONF_PLATFORM_PRI_CAPS@)
memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_CFLAGS       = \
	$(AM_CFLAGS_@VGCONF_PLATFORM_PRI_CAPS@)
memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_DEPENDENCIES = \
	$(TOOL_DEPENDENCIES_@VGCONF_PLATFORM_PRI_CAPS@)
memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_LDADD        = \
	$(TOOL_LDADD_@VGCONF_PLATFORM_PRI_CAPS@)
memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_LDFLAGS      = \
	$(TOOL_LDFLAGS_@VGCONF_PLATFORM_PRI_CAPS@)
memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_LINK = \
	$(top_builddir)/coregrind/link_tool_exe_@VGCONF_OS@ \
	@VALT_LOAD_ADDRESS_PRI@ \
	$(LINK) \
	$(memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_CFLAGS) \
	$(memadump_@VGCONF_ARCH_PRI@_@VGCONF_OS@_LDFLAGS)

if VGCONF_HAVE_PLATFORM_SEC
memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_SOURCES      = \
	$(NONE_SOURCES_COMMON)
memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_CPPFLAGS     = \
	$(AM_CPPFLAGS_@VGCONF_PLATFORM_SEC_CAPS@)
memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_CFLAGS       = \
	$(AM_CFLAGS_@VGCONF_PLATFORM_SEC_CAPS@)
memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_DEPENDENCIES = \
	$(TOOL_DEPENDENCIES_@VGCONF_PLATFORM_SEC_CAPS@)
memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_LDADD        = \
	$(TOOL_LDADD_@VGCONF_PLATFORM_SEC_CAPS@)
memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_LDFLAGS      = \
	$(TOOL_LDFLAGS_@VGCONF_PLATFORM_SEC_CAPS@)
memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_LINK = \
	$(top_builddir)/coregrind/link_tool_exe_@VGCONF_OS@ \
	@VALT_LOAD_ADDRESS_SEC@ \
	$(LINK) \
	$(memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_CFLAGS) \
	$(memadump_@VGCONF_ARCH_SEC@_@VGCONF_OS@_LDFLAGS)
endif


