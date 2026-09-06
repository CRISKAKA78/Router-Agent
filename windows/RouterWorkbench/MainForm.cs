using System.Text;
using System.Text.Json;
using RouterWorkbench.Core;

namespace RouterWorkbench;

public sealed partial class MainForm : Form
{
    private ServerProfile profile = new();
    private readonly string profilePath;
    private readonly Action<Endpoint, ServerProfile> openEndpoint;
    private WorkspaceConnection? connection;
    private Snapshot? snapshot;
    private readonly CancellationTokenSource lifetime = new();
    private readonly HashSet<Task> uiWork = [];
    private readonly HashSet<string> readsRunning = [], readsPending = [];
    private bool busy, switching, closing, closed, binding;
    private int taskRevision, toolRevision;
    private string? preferredMaintenance, preferredTask;
    private readonly TextBox serverUrl = new() { Width = 330, AccessibleName = "Server 地址" };
    private readonly Label connectionStatus = new() { AutoSize = true, Text = "未连接", ForeColor = Color.DimGray };
    private readonly Label errorStatus = new() { Dock = DockStyle.Fill, AutoEllipsis = true, ForeColor = Color.Firebrick, TextAlign = ContentAlignment.MiddleLeft };
    private readonly Label selectedTitle = new() { AutoSize = true, Text = "请选择设备", Font = new Font("Microsoft YaHei UI", 14, FontStyle.Bold) };
    private readonly TextBox search = new() { Width = 180, PlaceholderText = "搜索设备 / 型号 / 主机名" };
    private readonly CheckBox onlineOnly = new() { Text = "仅在线", AutoSize = true };
    private readonly DataGridView devices = Grid(("DeviceId", "设备", 165), ("Status", "状态", 65), ("Hostname", "主机名", 120));
    private readonly TabControl tabs = new() { Dock = DockStyle.Fill };
    private readonly RichTextBox deviceDetails = Output();
    private readonly DataGridView maintenance = Grid(("MaintenanceId", "Maintenance", 215), ("State", "状态", 85), ("ExpiresAt", "到期时间", 180), ("Reason", "关闭原因", 140));
    private readonly Label maintenanceInfo = new() { Dock = DockStyle.Fill, AutoEllipsis = true, Text = "开启远程维护后，在此直接打开 Web / SSH / Telnet。" };
    private readonly CheckBox defaultLease = new() { Text = "默认 240 分钟", Checked = true, AutoSize = true };
    private readonly TextBox lease = new() { Text = "240", Width = 110, Enabled = false, AccessibleName = "自定义租期" };
    private readonly ComboBox leaseUnit = new() { DropDownStyle = ComboBoxStyle.DropDownList, Width = 80, Enabled = false };
    private readonly Dictionary<string, TextBox> addresses = [];
    private readonly Dictionary<string, Button> launchButtons = [];
    private readonly List<Button> mutations = [];
    private readonly System.Windows.Forms.Timer countdown = new() { Interval = 1000 };
    private readonly TextBox command = new() { Width = 380, Text = "uname -a", AccessibleName = "Exec 命令" };
    private readonly NumericUpDown timeout = new() { Minimum = 1, Maximum = uint.MaxValue, Value = 30, Width = 85 };
    private readonly DataGridView tasks = Grid(("TaskId", "Task", 235), ("Type", "类型", 85), ("State", "状态", 95), ("CreatedAt", "创建时间", 175));
    private readonly RichTextBox taskOutput = Output();
    private readonly DataGridView assets = Grid(("AssetId", "资产 ID", 235), ("Name", "名称", 160), ("Size", "字节", 90), ("Archived", "已归档", 70));
    private readonly DataGridView toolsGrid = Grid(("ToolId", "工具 ID", 235), ("Name", "名称", 150), ("Archived", "已归档", 75));
    private readonly ComboBox versions = new() { DropDownStyle = ComboBoxStyle.DropDownList, Width = 200 };
    private readonly DataGridView artifacts = Grid(("ArtifactId", "产物 ID", 240), ("AssetId", "资产 ID", 235), ("Status", "兼容结果", 105), ("Mode", "权限", 65));
    private readonly RichTextBox compatibilityDetails = Output();
    private readonly Button retryButton = new() { Text = "重试原请求", AutoSize = true, Enabled = false };
    private readonly Button acknowledgeButton = new() { Text = "已核对不确定操作", AutoSize = true, Enabled = false };

    public MainForm(string? profilePath = null, Action<Endpoint, ServerProfile>? openEndpoint = null)
    {
        this.profilePath = profilePath ?? ServerProfile.DefaultPath;
        this.openEndpoint = openEndpoint ?? EndpointLauncher.Open;
        Text = "路由器远程维护工作台"; StartPosition = FormStartPosition.CenterScreen;
        Size = new Size(1390, 880); MinimumSize = new Size(1100, 720); Font = new Font("Microsoft YaHei UI", 9);
        AutoScaleMode = AutoScaleMode.Dpi;
        var root = new TableLayoutPanel { Dock = DockStyle.Fill, RowCount = 4, ColumnCount = 1, Padding = new Padding(12) };
        root.RowStyles.Add(new(SizeType.Absolute, 43)); root.RowStyles.Add(new(SizeType.Absolute, 28));
        root.RowStyles.Add(new(SizeType.Percent, 100)); root.RowStyles.Add(new(SizeType.Absolute, 65));
        var connectRow = Row();
        connectRow.Controls.Add(new Label { Text = "Management Server", AutoSize = true, Padding = new Padding(0, 7, 0, 0) });
        connectRow.Controls.Add(serverUrl);
        connectRow.Controls.Add(ActionButton("连接 / 切换", ConnectAsync));
        connectRow.Controls.Add(ActionButton("断开", DisconnectAsync));
        connectRow.Controls.Add(ActionButton("设置…", SettingsAsync));
        connectRow.Controls.Add(ActionButton("刷新", () => { connection?.Invalidate(); return Task.CompletedTask; }));
        root.Controls.Add(connectRow, 0, 0); root.Controls.Add(connectionStatus, 0, 1);
        var split = new SplitContainer { Dock = DockStyle.Fill, FixedPanel = FixedPanel.Panel1, SplitterDistance = 340, Size = new Size(1320, 650), Panel1MinSize = 280, Panel2MinSize = 650 };
        var left = Layout((SizeType.Absolute, 36), (SizeType.Percent, 100));
        var filter = Row(); filter.Controls.AddRange([search, onlineOnly]); left.Controls.Add(filter, 0, 0); left.Controls.Add(devices, 0, 1);
        split.Panel1.Controls.Add(left);
        var right = Layout((SizeType.Absolute, 40), (SizeType.Percent, 100)); right.Controls.Add(selectedTitle, 0, 0); right.Controls.Add(tabs, 0, 1); split.Panel2.Controls.Add(right);
        root.Controls.Add(split, 0, 2);
        var footer = Layout((SizeType.Absolute, 30), (SizeType.Absolute, 30)); footer.Controls.Add(errorStatus, 0, 0);
        var recovery = Row(); recovery.Controls.AddRange([retryButton, acknowledgeButton]); footer.Controls.Add(recovery, 0, 1); root.Controls.Add(footer, 0, 3);
        Controls.Add(root);
        BuildMaintenance(); BuildDevice(); BuildTasks(); BuildFiles(); BuildTools();
        search.TextChanged += (_, _) => BindDevices(); onlineOnly.CheckedChanged += (_, _) => BindDevices();
        devices.SelectionChanged += (_, _) => { if (!binding) DeviceSelected(); };
        maintenance.SelectionChanged += (_, _) => { if (!binding) RenderMaintenance(); };
        tasks.SelectionChanged += (_, _) => { if (!binding) Queue(ReadTaskAsync); };
        toolsGrid.SelectionChanged += (_, _) => { if (!binding) Queue(ReadToolsAsync); };
        versions.SelectedIndexChanged += (_, _) => { if (!binding) Queue(ReadCompatibilityAsync); };
        artifacts.SelectionChanged += (_, _) => ShowCompatibility();
        defaultLease.CheckedChanged += (_, _) => lease.Enabled = leaseUnit.Enabled = !defaultLease.Checked;
        countdown.Tick += (_, _) => RenderMaintenance(); countdown.Start();
        retryButton.Click += (_, _) => Queue(RetryAsync);
        acknowledgeButton.Click += (_, _) => {
            if (MessageBox.Show(this, "请先通过当前快照核对操作结果。如果 Server 已重启，旧幂等键无法防止再次执行。确认已核对后才清除此提示。", "核对不确定操作", MessageBoxButtons.OKCancel, MessageBoxIcon.Information) == DialogResult.OK)
            { connection?.AbandonPending(); errorStatus.Text = "已清除不确定请求；Server 当前快照仍为操作事实来源。"; UpdateActions(); }
        };
        Shown += (_, _) => {
            try { profile = ServerProfile.Load(this.profilePath); } catch (Exception e) { errorStatus.Text = Errors.Describe(e, "读取连接配置"); }
            serverUrl.Text = profile.ServerUrl;
        };
        FormClosing += OnClosing;
        UpdateActions();
    }
    private static DataGridView Grid(params (string Property, string Header, int Width)[] columns)
    {
        var grid = new DataGridView { Dock = DockStyle.Fill, ReadOnly = true, AllowUserToAddRows = false,
            AllowUserToDeleteRows = false, AutoGenerateColumns = false, MultiSelect = false, SelectionMode = DataGridViewSelectionMode.FullRowSelect,
            RowHeadersVisible = false, BackgroundColor = SystemColors.Window, BorderStyle = BorderStyle.FixedSingle, AutoSizeRowsMode = DataGridViewAutoSizeRowsMode.None };
        foreach (var (property, header, width) in columns) grid.Columns.Add(new DataGridViewTextBoxColumn { DataPropertyName = property, HeaderText = header, Width = width, SortMode = DataGridViewColumnSortMode.NotSortable });
        grid.CellFormatting += (_, e) => { if (e.Value is DateTimeOffset time) { e.Value = time.LocalDateTime.ToString("yyyy-MM-dd HH:mm:ss"); e.FormattingApplied = true; } };
        return grid;
    }
    private static RichTextBox Output() => new() { Dock = DockStyle.Fill, ReadOnly = true, BackColor = SystemColors.Window, Font = new Font("Consolas", 10), DetectUrls = false, WordWrap = false };
    private static FlowLayoutPanel Row() => new() { Dock = DockStyle.Fill, AutoSize = true, WrapContents = true, Padding = new Padding(0, 2, 0, 2) };
    private new static TableLayoutPanel Layout(params (SizeType Type, float Size)[] rows)
    {
        var panel = new TableLayoutPanel { Dock = DockStyle.Fill, ColumnCount = 1, RowCount = rows.Length };
        foreach (var (type, size) in rows) panel.RowStyles.Add(new(type, size));
        return panel;
    }
    private TabPage Page(string title, Control content) { var page = new TabPage(title) { Padding = new Padding(10) }; page.Controls.Add(content); tabs.TabPages.Add(page); return page; }
    private Button ActionButton(string text, Func<Task> action, bool mutation = false)
    {
        var button = new Button { Text = text, AutoSize = true, MinimumSize = new Size(82, 28) };
        var lastClick = long.MinValue;
        button.Click += (_, _) => {
            var now = Environment.TickCount64;
            if (mutation && lastClick != long.MinValue && now - lastClick <= SystemInformation.DoubleClickTime) return;
            lastClick = now; Queue(action);
        };
        if (mutation) mutations.Add(button);
        return button;
    }
    private void Queue(Func<Task> operation)
    {
        if (closing) return;
        var task = GuardAsync(operation); uiWork.Add(task);
        _ = task.ContinueWith(_ => { if (!IsDisposed && IsHandleCreated) BeginInvoke(() => uiWork.Remove(task)); }, TaskScheduler.Default);
    }
    private async Task GuardAsync(Func<Task> operation)
    {
        try { await operation(); }
        catch (OperationCanceledException) when (closing) { }
        catch (Exception e) { if (!closing) { errorStatus.Text = Errors.Describe(e, "操作失败"); UpdateActions(); } }
    }
    private async Task CoalesceReadAsync(string key, Func<Task> read)
    {
        readsPending.Add(key);
        if (!readsRunning.Add(key)) return;
        try
        {
            do { readsPending.Remove(key); await read(); }
            while (readsPending.Contains(key) && !closing);
        }
        finally { readsRunning.Remove(key); readsPending.Remove(key); }
    }
    private void Dispatch(WorkspaceConnection owner, Action action)
    {
        if (closing || IsDisposed || !IsHandleCreated) return;
        try { BeginInvoke(() => { if (!closing && ReferenceEquals(connection, owner)) action(); }); }
        catch (InvalidOperationException) when (closing || IsDisposed) { }
    }
    private async Task ConnectAsync()
    {
        if (switching) return;
        var next = profile with { ServerUrl = serverUrl.Text.Trim() }; next.BaseUri();
        switching = true;
        try
        {
            await DisconnectCoreAsync();
            if (closing) return;
            profile = next;
            await profile.SaveAsync(profilePath);
            if (closing) return;
            var owner = new WorkspaceConnection(profile); connection = owner;
            owner.ConnectionChanged += text => Dispatch(owner, () => { connectionStatus.Text = text; UpdateActions(); });
            owner.RefreshFailed += text => Dispatch(owner, () => errorStatus.Text = text);
            owner.SnapshotChanged += state => Dispatch(owner, () => ApplySnapshot(state));
            owner.Start();
        }
        finally { switching = false; UpdateActions(); }
    }
    private async Task DisconnectAsync() { if (switching) return; switching = true; try { await DisconnectCoreAsync(); } finally { switching = false; } }
    private async Task DisconnectCoreAsync()
    {
        var old = connection; connection = null; snapshot = null; taskRevision++; toolRevision++;
        preferredMaintenance = preferredTask = null;
        binding = true;
        foreach (var grid in new[] { devices, tasks, maintenance, assets, toolsGrid, artifacts }) grid.DataSource = null;
        versions.DataSource = null; binding = false;
        deviceDetails.Clear(); taskOutput.Clear(); compatibilityDetails.Clear(); selectedTitle.Text = "请选择设备";
        errorStatus.Text = ""; connectionStatus.Text = "未连接"; RenderMaintenance(); UpdateActions();
        if (old != null) await old.DisposeAsync();
    }
    private async Task SettingsAsync()
    {
        using var dialog = new SettingsDialog(profile);
        if (dialog.ShowDialog(this) != DialogResult.OK) return;
        profile = dialog.Apply(profile); await profile.SaveAsync(profilePath);
        errorStatus.Text = "外部客户端设置已保存。";
    }
    private async void OnClosing(object? sender, FormClosingEventArgs e)
    {
        if (closed) return;
        e.Cancel = true; if (closing) return; closing = true;
        countdown.Stop(); lifetime.Cancel(); Enabled = false;
        try { await DisconnectCoreAsync(); await Task.WhenAll(uiWork.ToArray()); }
        finally { countdown.Dispose(); lifetime.Dispose(); closed = true; Close(); }
    }
    private string? DeviceId => (devices.CurrentRow?.DataBoundItem as DeviceRow)?.DeviceId;
    private Device? CurrentDevice => snapshot?.Devices.FirstOrDefault(d => d.DeviceId == DeviceId);
    private Maintenance? CurrentMaintenance => maintenance.CurrentRow?.DataBoundItem as Maintenance;
    private TaskSummary? CurrentTask => tasks.CurrentRow?.DataBoundItem as TaskSummary;
    private Asset? CurrentAsset => assets.CurrentRow?.DataBoundItem as Asset;
    private Tool? CurrentTool => toolsGrid.CurrentRow?.DataBoundItem as Tool;
    private ToolVersion? CurrentVersion => versions.SelectedItem as ToolVersion;
    private WorkspaceConnection RequireConnection() => connection is { IsSynchronized: true } c ? c : throw new InvalidOperationException("请等待 Server 快照同步后再操作。");
    private string RequireDevice() => CurrentDevice is { Status: "online" } d ? d.DeviceId : throw new ApiException("device_offline", "请选择在线设备。", 409);
    private void UpdateActions()
    {
        var enabled = !closing && !busy && connection is { IsSynchronized: true, Pending: null };
        foreach (var button in mutations) button.Enabled = enabled;
        retryButton.Enabled = !busy && !closing && connection?.Pending != null;
        acknowledgeButton.Enabled = retryButton.Enabled;
        RenderMaintenance();
    }
    private async Task ExecuteAsync(Mutation request, bool retry = false)
    {
        if (busy) return;
        var owner = RequireConnection(); busy = true; errorStatus.Text = request.Label + "…"; UpdateActions();
        try
        {
            var result = await owner.ExecuteAsync(request, retry);
            if (!ReferenceEquals(owner, connection) || closing) return;
            var message = request.Label + "：已收到 Server 响应。";
            if (result.TryGetProperty("task_id", out var id)) { preferredTask = id.GetString(); message += " Task " + preferredTask; }
            if (result.TryGetProperty("maintenance_id", out var maintenanceId)) preferredMaintenance = maintenanceId.GetString();
            if (result.TryGetProperty("dispatch_uncertain", out var uncertain) && uncertain.GetBoolean()) message += "；派发不确定，已保留 Task，请查看结果，不要新建替代任务。";
            if (result.TryGetProperty("warning", out var warning)) message += "；Server 提示：" + warning.GetString();
            errorStatus.Text = message;
        }
        catch (Exception e)
        {
            if (ReferenceEquals(owner, connection) && !closing)
            {
                errorStatus.Text = Errors.Describe(e, request.Label);
                if (owner.Pending != null) errorStatus.Text += " 响应不确定；核对快照后可重试原请求，仅限 Server 未重启。";
            }
        }
        finally { busy = false; if (!closing) UpdateActions(); }
    }
    private async Task RetryAsync()
    {
        if (connection?.Pending is not { } request) return;
        if (MessageBox.Show(this, "仅在确认 Management Server 未重启时重试。将使用相同幂等键和原始请求；Server 重启后不能保证去重。", "重试原请求", MessageBoxButtons.OKCancel, MessageBoxIcon.Information) == DialogResult.OK)
            await ExecuteAsync(request, true);
    }
    private void ApplySnapshot(Snapshot state)
    {
        var oldSession = CurrentDevice?.CurrentSession?.SessionId;
        var nextSession = state.Devices.FirstOrDefault(d => d.DeviceId == DeviceId)?.CurrentSession?.SessionId;
        if (oldSession != null && nextSession != null && oldSession != nextSession)
            errorStatus.Text = "Session 已替换；设备资料已刷新，请核对当前 Session 和 Maintenance。";
        snapshot = state; BindDevices();
        binding = true;
        Bind(assets, state.Assets, a => a.AssetId); Bind(toolsGrid, state.Tools, t => t.ToolId);
        binding = false;
        Queue(ReadToolsAsync); UpdateActions();
    }
    private static void Bind<T>(DataGridView grid, T[] rows, Func<T, string> key)
    {
        var selected = grid.CurrentRow?.DataBoundItem is T item ? key(item) : null;
        var old = grid.DataSource as T[];
        // Keep focus and scroll during periodic refresh when the visible rows are unchanged.
        if (old != null && old.SequenceEqual(rows)) return;
        var scroll = grid.FirstDisplayedScrollingRowIndex;
        grid.DataSource = rows;
        if (selected != null)
            foreach (DataGridViewRow row in grid.Rows)
                if (row.DataBoundItem is T value && key(value) == selected) { grid.CurrentCell = row.Cells[0]; break; }
        if (scroll >= 0 && scroll < grid.Rows.Count) grid.FirstDisplayedScrollingRowIndex = scroll;
    }
    private sealed record DeviceRow(string DeviceId, string Status, string Hostname);
    private void BindDevices()
    {
        binding = true;
        var query = search.Text.Trim();
        var filtered = snapshot?.Devices.Where(d => (!onlineOnly.Checked || d.Status == "online") &&
            (d.DeviceId.Contains(query, StringComparison.OrdinalIgnoreCase) || d.Registration.Hostname.Contains(query, StringComparison.OrdinalIgnoreCase) || d.Registration.Model.Contains(query, StringComparison.OrdinalIgnoreCase))) ?? [];
        Bind(devices, filtered.Select(d => new DeviceRow(d.DeviceId, d.Status, d.Registration.Hostname)).ToArray(), d => d.DeviceId);
        binding = false; DeviceSelected();
    }
    private void DeviceSelected()
    {
        var device = CurrentDevice;
        selectedTitle.Text = device == null ? "请选择设备" : $"{device.DeviceId}   ·   {device.Status}";
        deviceDetails.Text = device == null ? "" : FormatDevice(device);
        binding = true;
        Bind(maintenance, snapshot?.Maintenance.Where(m => m.DeviceId == DeviceId).OrderByDescending(m => m.CreatedAt).ToArray() ?? [], m => m.MaintenanceId);
        Bind(tasks, snapshot?.Tasks.Where(t => t.DeviceId == DeviceId).OrderByDescending(t => t.CreatedAt).ToArray() ?? [], t => t.TaskId);
        SelectPreferred(maintenance, ref preferredMaintenance, item => ((Maintenance)item).MaintenanceId);
        SelectPreferred(tasks, ref preferredTask, item => ((TaskSummary)item).TaskId);
        binding = false; RenderMaintenance(); Queue(ReadTaskAsync); Queue(ReadCompatibilityAsync);
    }
    private static void SelectPreferred(DataGridView grid, ref string? preferred, Func<object, string> key)
    {
        if (preferred == null) return;
        foreach (DataGridViewRow row in grid.Rows)
            if (row.DataBoundItem != null && key(row.DataBoundItem) == preferred) { grid.CurrentCell = row.Cells[0]; preferred = null; return; }
    }
    private static string FormatDevice(Device d)
    {
        var r = d.Registration;
        return $"设备：{d.DeviceId}\n状态：{d.Status}\n当前 Session：{d.CurrentSession?.SessionId ?? "无"}\n最近 Session：{d.LatestSession?.SessionId}\n最近结束原因：{d.LatestSession?.EndReason}\n\n主机：{r.Hostname}\n序列号：{r.Serial}\n型号：{r.Model}\n固件：{r.Firmware}\nProbe：{r.ProbeVersion}\n架构：{r.Arch}\n内核：{r.Kernel}\nlibc：{r.Libc}\n能力：{string.Join(", ", r.Capabilities ?? [])}\n\n首次发现：{d.FirstSeenAt}\n最近活动：{d.LastSeenAt}\n最近上线：{d.LastOnlineAt}\n最近离线：{d.LastOfflineAt}\n累计 Session：{d.TotalSessions}，已淘汰历史：{d.EvictedSessions}";
    }
}
