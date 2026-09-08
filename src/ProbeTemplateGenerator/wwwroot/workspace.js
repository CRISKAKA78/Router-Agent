// Browser capabilities only. Project state, validation and compilation live in C#.
window.workspace = (() => {
    let target;
    let dirty = false;
    let focusBeforeDialog;
    let dialogWasOpen = false;
    const dialogs = new MutationObserver(() => {
        const open = !!document.querySelector('[role="dialog"]');
        if (dialogWasOpen && !open && focusBeforeDialog?.isConnected) focusBeforeDialog.focus();
        dialogWasOpen = open;
    });
    const themeQuery = matchMedia('(prefers-color-scheme: dark)');
    let theme = 'Default';
    try { theme = localStorage.getItem('probe-generator.theme') || theme; } catch { /* Storage errors are surfaced by the C# draft service. */ }
    function applyTheme() {
        const dark = theme === 'Dark' || (theme === 'Default' && themeQuery.matches);
        document.documentElement.dataset.theme = dark ? 'dark' : 'light';
        document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
    }
    function keyboard(event) {
        const dialog = document.querySelector('[role="dialog"]');
        if (dialog && event.key === 'Tab') {
            const controls = [...dialog.querySelectorAll('button, fluent-button, input, select, textarea, a[href], [tabindex="0"]')]
                .filter(element => !element.disabled && !element.hasAttribute('disabled') && element.getClientRects().length);
            const active = document.activeElement;
            if (controls.length && event.shiftKey && active === controls[0]) {
                event.preventDefault(); controls.at(-1).focus();
            } else if (controls.length && !event.shiftKey && active === controls.at(-1)) {
                event.preventDefault(); controls[0].focus();
            }
        }
        if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
            event.preventDefault();
            target?.invokeMethodAsync('SaveShortcut');
        }
    }
    function beforeUnload(event) {
        if (dirty) { event.preventDefault(); event.returnValue = ''; }
    }
    function menus(event) {
        if (!document.querySelector('[role="dialog"]')) focusBeforeDialog = document.activeElement;
        for (const menu of document.querySelectorAll('details.more-menu[open], details.add-menu[open]')) {
            if (!menu.contains(event.target) || event.target.closest('button')) menu.open = false;
        }
    }
    function menuChange(event) {
        event.target.closest('details.more-menu')?.removeAttribute('open');
    }
    themeQuery.addEventListener('change', applyTheme);
    applyTheme();
    return {
        initialize(reference) {
            target = reference;
            dialogs.observe(document.body, {childList:true, subtree:true});
            document.removeEventListener('keydown', keyboard);
            document.addEventListener('keydown', keyboard);
            document.addEventListener('click', menus);
            document.addEventListener('change', menuChange);
            window.addEventListener('beforeunload', beforeUnload);
        },
        dispose() {
            target = undefined;
            dialogs.disconnect();
            document.removeEventListener('keydown', keyboard);
            document.removeEventListener('click', menus);
            document.removeEventListener('change', menuChange);
            window.removeEventListener('beforeunload', beforeUnload);
        },
        setDirty(value) { dirty = value; },
        setTheme(value) { theme = value; applyTheme(); try { localStorage.setItem('probe-generator.theme', value); } catch { /* Keep appearance usable when browser storage is unavailable. */ } },
        storageGet(key) { return localStorage.getItem(key); },
        storageSet(key, value) { localStorage.setItem(key, value); },
        download(name, text) {
            const url = URL.createObjectURL(new Blob([text], { type: 'application/json;charset=utf-8' }));
            const anchor = document.createElement('a');
            anchor.download = name.replace(/[\\/:*?"<>|\x00-\x1f]/g, '_');
            anchor.href = url;
            document.body.append(anchor);
            anchor.click();
            anchor.remove();
            setTimeout(() => URL.revokeObjectURL(url), 10000);
        }
    };
})();
