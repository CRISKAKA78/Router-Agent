using System.ComponentModel;

namespace RouterWorkbench.Desktop;

internal sealed class QuickProperty(string name) : INotifyPropertyChanged
{
    public string Name { get; } = name;
    private string value = "";
    public string Value
    {
        get => value;
        set {
            if (this.value == value) return;
            this.value = value;
            PropertyChanged?.Invoke(this, new(nameof(Value)));
        }
    }
    public event PropertyChangedEventHandler? PropertyChanged;
}
