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
    const widths = new WeakMap();
    function applyColumns(table, state) {
        table.style.tableLayout = 'fixed';
        table.style.width = state.sizes.reduce((a,b) => a+b, 0) + 'px';
        for (const row of table.rows) [...row.cells].forEach((cell, i) => {
            if (cell.colSpan !== 1 || !state.sizes[i]) return;
            cell.style.width = state.sizes[i] + 'px';
            cell.classList.toggle('column-wrap', state.wrapped.has(i));
        });
    }
    function prepareTables() {
        for (const table of document.querySelectorAll('.data-table')) {
            for (const header of table.tHead?.rows[0]?.cells || []) {
                header.tabIndex = 0;
                header.title = header.textContent.trim() + '（拖动右边缘调整列宽；方向键调整）';
            }
            for (const cell of table.querySelectorAll('td')) if (!cell.querySelector('input,select,button')) cell.title = cell.textContent.trim();
            if (widths.has(table)) applyColumns(table, widths.get(table));
        }
    }
    const tables = new MutationObserver(prepareTables);
    function columnState(table) {
        if (!widths.has(table)) widths.set(table, {sizes:[...table.tHead.rows[0].cells].map(h=>h.getBoundingClientRect().width), wrapped:new Set()});
        return widths.get(table);
    }
    function resizeColumn(table, state, index, size) {
        const width = Math.max(70, size);
        if (width < state.sizes[index]) state.wrapped.add(index);
        state.sizes[index] = width;
        applyColumns(table, state);
    }
    function tablePointer(event) {
        const header = event.target.closest('.data-table th');
        if (!header || header.getBoundingClientRect().right - event.clientX > 9) return;
        event.preventDefault();
        const table = header.closest('table'), state = columnState(table), index = header.cellIndex;
        const start = event.clientX, width = state.sizes[index];
        const move = e => resizeColumn(table, state, index, width + e.clientX - start);
        const end = () => { document.removeEventListener('pointermove',move); document.removeEventListener('pointerup',end); };
        document.addEventListener('pointermove',move);
        document.addEventListener('pointerup',end,{once:true});
    }
    function tableKey(event) {
        const header = event.target.closest('.data-table th');
        if (!header || !['ArrowLeft','ArrowRight'].includes(event.key)) return;
        event.preventDefault();
        const table = header.closest('table'), state = columnState(table);
        resizeColumn(table,state,header.cellIndex,state.sizes[header.cellIndex]+(event.key==='ArrowLeft'?-20:20));
    }
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
            tables.observe(document.body, {childList:true, subtree:true});
            prepareTables();
            document.addEventListener('pointerdown',tablePointer);
            document.addEventListener('keydown',tableKey);
            document.removeEventListener('keydown', keyboard);
            document.addEventListener('keydown', keyboard);
            document.addEventListener('click', menus);
            document.addEventListener('change', menuChange);
            window.addEventListener('beforeunload', beforeUnload);
        },
        dispose() {
            target = undefined;
            dialogs.disconnect();
            tables.disconnect();
            document.removeEventListener('pointerdown',tablePointer);
            document.removeEventListener('keydown',tableKey);
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
