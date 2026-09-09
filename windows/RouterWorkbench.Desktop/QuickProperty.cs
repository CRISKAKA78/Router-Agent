using System.ComponentModel;

namespace RouterWorkbench.Desktop;

internal sealed class QuickProperty(string name) : INotifyPropertyChanged
{
    public string Name { get; } = name;
    public bool HasTemplateUpdate { get; private set; }
    public bool CanUpdateTemplate { get; private set; }
    public void UpdateTemplateAction(bool visible, bool enabled)
    {
        if (HasTemplateUpdate != visible) { HasTemplateUpdate = visible; PropertyChanged?.Invoke(this, new(nameof(HasTemplateUpdate))); }
        if (CanUpdateTemplate != enabled) { CanUpdateTemplate = enabled; PropertyChanged?.Invoke(this, new(nameof(CanUpdateTemplate))); }
    }
    private string tip = "";
    public string ValueTip {get=>tip;set{if(tip==value)return;tip=value;PropertyChanged?.Invoke(this,new(nameof(ValueTip)));}}
    private string value = "";
    public string Value
    {
        get => value;
        set {
            if(this.value==value)return;
            this.value = value;
            PropertyChanged?.Invoke(this, new(nameof(Value)));
        }
    }
    public event PropertyChangedEventHandler? PropertyChanged;
}
