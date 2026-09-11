"""Render the actual WPF vector output for visual QA when remote desktop capture is unavailable."""
from pathlib import Path
import sys
import re
import fitz

root = Path(sys.argv[1] if len(sys.argv) > 1 else "build/windows-desktop/verification")
files = sorted(root.glob("*.xps"))
if not files:
    raise SystemExit("No WPF XPS renders found")
for path in files:
    with fitz.open(path) as doc:
        assert len(doc) == 1, f"Unexpected WPF pagination: {path}"
        text = doc[0].get_text()
        if path.stem.startswith("interface-sampling-"):
            assert all(label in text for label in ["采样周期", "采样接口", "接口名称", "恢复模板默认", "保存", "取消"]), f"Incomplete sampling dialog: {path}"
        elif path.stem == "RenameDevice":
            assert "设备名称" in text and "已重命名设备" in text and "确定" in text, f"Incomplete rename dialog: {path}"
        elif path.stem == "UpdateDeviceTemplate":
            assert "当前生效" in text and "应用版本" in text and "应用" in text and "取消" in text, f"Incomplete template confirmation: {path}"
        elif path.stem == "tool-deploy":
            assert all(label in text for label in ["工具版本", "目标目录", "目标文件", "确认投放", "取消"]), f"Incomplete tool dialog: {path}"
        elif path.stem.startswith("font-chart-"):
            assert all(label in text for label in ["B/s", "接收", "发送", "等待有效速率采样"]), f"Incomplete large-font chart: {path}"
            assert len(re.findall(r"\d{2}:\d{2}", text)) >= 3 and len(doc[0].get_drawings()) >= 5, f"Missing large-font axes: {path}"
        elif path.stem.startswith("network-chart-"):
            assert "B/s" in text and "接收" in text and "发送" in text, f"Missing chart units or legend: {path}"
            assert len(re.findall(r"\d{2}:\d{2}", text)) >= 6 and len(doc[0].get_drawings()) >= 8, f"Missing chart axes or curves: {path}"
        elif path.stem.startswith("networks-empty-"):
            assert all(label in text for label in ["新建网络", "添加设备", "应用 / 启动", "停止组网", "回查不确定操作", "成员", "拓扑", "链路", "操作记录"]), f"Incomplete overlay workspace: {path}"
        elif path.stem == "networks-topology-dark":
            assert all(label in text for label in ["观测连接", "厂区 A", "厂区 B", "引擎运行", "未知 / 已过期"]), f"Incomplete observed topology: {path}"
            assert len(doc[0].get_drawings()) >= 7, f"Missing observed nodes/edge: {path}"
        elif path.stem == "networks-logical-light":
            assert all(label in text for label in ["逻辑成员关系", "不表示实际直连", "Peer 1", "Peer 2"]), f"Incomplete logical topology: {path}"
            assert len(doc[0].get_drawings()) >= 6, f"Missing logical nodes: {path}"
        else:
            assert len(text) > 150, f"Empty WPF vector output: {path}"
        pixmap = doc[0].get_pixmap(matrix=fitz.Matrix(4/3, 4/3), alpha=False)
        assert len(set(pixmap.samples)) > 20, f"Blank rendered page: {path}"
        pixmap.save(str(path.with_suffix(".png")))
    print(f"PASS WPF vector render {path.stem}")
print(f"Rendered {len(files)} actual WPF layouts; excludes native title bar.")
