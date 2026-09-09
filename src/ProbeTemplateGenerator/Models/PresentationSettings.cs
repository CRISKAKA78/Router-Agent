using System.Text.Json;
using System.Text.Json.Serialization;
namespace ProbeTemplateGenerator.Models;
public sealed class DisplayGroup { [JsonPropertyName("id")]public string Id{get;set;}="";[JsonPropertyName("name")]public string Name{get;set;}="";[JsonPropertyName("order")]public int Order{get;set;}=10; }
[JsonUnmappedMemberHandling(JsonUnmappedMemberHandling.Disallow)]
public sealed class DisplayField {
 [JsonPropertyName("group_id")]public string GroupId{get;set;}="";
 [JsonPropertyName("order")]public int Order{get;set;}=10;
 [JsonPropertyName("visible"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)]public bool? Visible{get;set;}
}
[JsonUnmappedMemberHandling(JsonUnmappedMemberHandling.Disallow)]
public sealed class PresentationSettings {
 [JsonPropertyName("storage_visible"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)]public bool? StorageVisible{get;set;}
 [JsonPropertyName("groups")]public List<DisplayGroup> Groups{get;set;}=[];
 [JsonPropertyName("fields")]public Dictionary<string,DisplayField> Fields{get;set;}=[];
 [JsonPropertyName("builtin_visibility")]public Dictionary<string,bool> BuiltinVisibility{get;set;}=[];
 public PresentationSettings Copy()=>JsonSerializer.Deserialize<PresentationSettings>(JsonSerializer.Serialize(this))!;
}
public sealed class SwitchPort {
 [JsonPropertyName("id")]public string Id{get;set;}="";
 [JsonPropertyName("switch_id")]public string SwitchId{get;set;}="";
 [JsonPropertyName("role")]public string Role{get;set;}="external";
 [JsonPropertyName("port"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)]public int? Port{get;set;}
 [JsonPropertyName("system_name")]public string SystemName{get;set;}="";
 [JsonPropertyName("uplink")]public string Uplink{get;set;}="";
 [JsonPropertyName("display_name")]public string DisplayName{get;set;}="";
}
public sealed class SwitchSettings {
 [JsonPropertyName("counters"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)]public PortCounters? Counters{get;set;}
 [JsonPropertyName("backend")]public string Backend{get;set;}="auto";
 [JsonPropertyName("command"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)]public string? Command{get;set;}
 [JsonPropertyName("ports")]public List<SwitchPort> Ports{get;set;}=[];
 public SwitchSettings Copy()=>JsonSerializer.Deserialize<SwitchSettings>(JsonSerializer.Serialize(this))!;
}
public sealed class PortCounters {
 [JsonPropertyName("backend")]public string Backend{get;set;}="swconfig_mib";
 [JsonPropertyName("command"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)]public string? Command{get;set;}
 [JsonPropertyName("rx_field"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)]public string? RxField{get;set;}="RxGoodByte";
 [JsonPropertyName("tx_field"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)]public string? TxField{get;set;}="TxByte";
 [JsonPropertyName("bits")]public int Bits{get;set;}=64;
 [JsonPropertyName("basis")]public string Basis{get;set;}="RX好帧字节 / TX字节；FCS等帧开销未确认";
}
public sealed class DeviceModel {
 [JsonPropertyName("model_id")]public string ModelId{get;set;}=Guid.NewGuid().ToString("N");
 [JsonPropertyName("version")]public ulong Version{get;set;}
 [JsonPropertyName("name")]public string Name{get;set;}="";
 [JsonPropertyName("aliases")]public string[] Aliases{get;set;}=[];
 [JsonPropertyName("template_id")]public string TemplateId{get;set;}="";
}
