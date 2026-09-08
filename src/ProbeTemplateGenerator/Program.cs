using Microsoft.FluentUI.AspNetCore.Components;
using ProbeTemplateGenerator.Components;
using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Persistence;
using ProbeTemplateGenerator.Services;

var builder = WebApplication.CreateBuilder(args);
// The local launcher runs the compiled app from source in Production; enable referenced
// Blazor/Fluent assets there as well as in a conventional published directory.
builder.WebHost.UseStaticWebAssets();
builder.Services.AddRazorComponents().AddInteractiveServerComponents();
builder.Services.AddFluentUIComponents();
builder.Services.AddScoped<TemplateCompiler>();
builder.Services.AddScoped<ProjectFiles>();
builder.Services.AddScoped<WorkspacePersistence>();
builder.Services.AddScoped<EditorWorkspace>();
builder.Services.AddHttpClient("templates", client => client.Timeout = TimeSpan.FromSeconds(40))
    .ConfigurePrimaryHttpMessageHandler(() => new HttpClientHandler { AllowAutoRedirect = false, UseCookies = false });
builder.Services.AddScoped(services => new TemplatePublishingService(
    services.GetRequiredService<IHttpClientFactory>().CreateClient("templates")));
var app = builder.Build();
app.UseAntiforgery();
app.MapStaticAssets();
app.MapRazorComponents<App>().AddInteractiveServerRenderMode();
if (Environment.GetEnvironmentVariable("PROBE_GENERATOR_OPEN_BROWSER") == "1")
    app.Lifetime.ApplicationStarted.Register(() =>
    {
        var url = app.Urls.FirstOrDefault(value => value.StartsWith("http://127.0.0.1:", StringComparison.Ordinal));
        if (url is not null)
            try { System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(url) { UseShellExecute = true }); }
            catch (Exception error) { app.Logger.LogWarning(error, "Open {Url} in a browser", url); }
    });
app.Run();

public partial class Program;
