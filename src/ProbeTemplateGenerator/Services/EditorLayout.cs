using ProbeTemplateGenerator.Models;

namespace ProbeTemplateGenerator.Services;

public sealed partial class EditorWorkspace
{
    public int ConfigurationTab { get; private set; }
    public string PresentationCategory {get;set;}="system";
    public void ConfigureField(string key){PresentationCategory=RouterWorkbench.Display.DevicePresentation.EditCategory(key);Configure(1);}
    private readonly Dictionary<string,string> pendingLayoutKeys = new();
    private PresentationSettings Layout => Project.Presentation ??= new();
    public void Configure(int tab) { ConfigurationTab=Math.Clamp(tab,0,4); SetStage(0); }
    public void SetVisible(string key,bool? visible) {
        if(Locked)return;
        var field=Placement(key);field.Visible=visible;Layout.Fields[key]=field;Touch();
    }
    public void SetCategoryVisible(string category,bool visible) {
        if(Locked)return;
        Layout.BuiltinVisibility[category]=visible;Touch();
    }
    public void SetStorageVisible(bool visible) { if(Locked)return; Layout.StorageVisible=visible;Touch(); }
    public int? VisibleOrder(string key)=>Project.Presentation?.Fields.TryGetValue(key,out var value)==true?value.Order:null;
    public DisplayField Placement(string key) => DisplayLayout.Placement(Project,key);
    public void SetGroup(string key,string group)
    {
        if(Locked || key.Length==0)return;
        var field=Placement(key); field.GroupId=group;
        field.Order=NextOrder(group,key); Layout.Fields[key]=field; Touch();
    }
    public void SetOrder(string key,int order)
    {
        if(Locked || key.Length==0)return;
        var field=Placement(key);field.Order=order;Layout.Fields[key]=field;Touch();
    }
    private int NextOrder(string group,string? except=null)
    {
        var rows=DisplayLayout.InGroup(Project,group).Where(f=>f.Key!=except).ToArray();
        var max=rows.Select(f=>Placement(f.Key).Order).DefaultIfEmpty(0).Max();
        if(max<999990)return max+10;
        for(var i=0;i<rows.Length;i++){var field=Placement(rows[i].Key);field.Order=(i+1)*10;Layout.Fields[rows[i].Key]=field;}
        return (rows.Length+1)*10;
    }
    public void AddGroup()
    {
        if(Locked||Layout.Groups.Count>=64)return;
        var n=1;while(Layout.Groups.Any(g=>g.Id=="group_"+n||g.Name=="分组"+n))n++;
        Layout.Groups.Add(new(){Id="group_"+n,Name="分组"+n,Order=Math.Min(1000000,Layout.Groups.Select(g=>g.Order).DefaultIfEmpty(0).Max()+10)});Touch();
    }
    public void RemoveGroup(DisplayGroup group)
    {
        if(Locked||DisplayLayout.IsBuiltinGroup(group.Id))return;
        Layout.Groups.Remove(group);foreach(var field in Layout.Fields.Values.Where(f=>f.GroupId==group.Id))field.GroupId="other";Touch();
    }
    public void MoveGroup(DisplayGroup group,int offset)
    {
        if(Locked)return;
        var groups=DisplayLayout.CustomGroups(Project).ToList();var i=groups.IndexOf(group);if(i<0||i+offset<0||i+offset>=groups.Count)return;
        (groups[i],groups[i+offset])=(groups[i+offset],groups[i]);for(var n=0;n<groups.Count;n++)groups[n].Order=(n+1)*10;Touch();
    }
    public void MoveField(string key,int offset)
    {
        if(Locked)return;
        var rows=DisplayLayout.InGroup(Project,DisplayLayout.GroupId(Project,key)).ToList();var i=rows.FindIndex(f=>f.Key==key);
        if(i<0||i+offset<0||i+offset>=rows.Count)return;
        (rows[i],rows[i+offset])=(rows[i+offset],rows[i]);
        for(var n=0;n<rows.Count;n++){var f=Placement(rows[n].Key);f.Order=(n+1)*10;Layout.Fields[rows[n].Key]=f;}Touch();
    }
    public void AddDeviceField(string key)
    {
        if(Locked||!System.Text.RegularExpressions.Regex.IsMatch(key,"^[a-z][a-z0-9_]{0,63}$")) { ShowToast("请填写有效的设备属性标识。",true);return; }
        var group=DisplayLayout.DefaultGroup(key);Layout.Fields.TryAdd(key,new(){GroupId=group,Order=NextOrder(group)});Touch();
    }
    public void ChangeAttributeKey(TemplateAttribute row,string value)
    {
        if(Locked)return;
        var previous=pendingLayoutKeys.GetValueOrDefault(row.Id,row.Key);row.Key=value;
        if(Project.Presentation?.Fields.TryGetValue(previous,out var layout)==true && previous!=value)
        {
            if(System.Text.RegularExpressions.Regex.IsMatch(value,"^[a-z][a-z0-9_]{0,63}$") && !Project.Attributes.Any(a=>a!=row&&a.Key==value))
            {Project.Presentation.Fields.Remove(previous);Project.Presentation.Fields[value]=layout;pendingLayoutKeys.Remove(row.Id);}
            else pendingLayoutKeys[row.Id]=previous;
        }
        Touch();
    }
    public void ChangeVisibility(TemplateAttribute row,AttributeVisibility visibility)
    {
        if(Locked)return;row.Visibility=visibility;
        if(visibility==AttributeVisibility.Virtual)Project.Presentation?.Fields.Remove(row.Key);Touch();
    }
}
