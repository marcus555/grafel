<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.csharp.framework.net-maui` — .NET MAUI

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C#](../by-language/csharp.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Mobile
- **Capability cells:** 37

## Capabilities


### Structure

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Context extraction | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_maui_edges_test.go`<br>`internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go` | Application.Current global context, DependencyService.Get<T>() resolution, IServiceProvider/MauiAppBuilder usages as SCOPE.Component/context_extraction. MauiProgram.cs DI registrations emit binding edges: builder.Services.AddSingleton/AddScoped<IFoo,Foo>() -> BINDS impl:Foo (interface->impl, with lifetime prop); AddSingleton/AddTransient/AddScoped<XViewModel>() -> REGISTERS impl:XViewModel (self-binding). Cross-file impl resolution is partial. |

### Navigation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Deep link extraction | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go` | Shell.Current.GoToAsync() deep-link routes, [QueryProperty] shell parameter bindings, and AppLinks.RegisterRoute/AppendAppLink registrations emitted as SCOPE.Pattern/deep_link_extraction. |
| Navigation extraction | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_maui_edges_test.go`<br>`internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go` | Navigation.PushAsync/PopAsync/PushModalAsync stack ops, NavigationPage/TabbedPage/FlyoutPage ctors detected. Shell routing emits NAVIGATES_TO edges to synthetic route:<path> stubs: Routing.RegisterRoute("r", typeof(Page)) and <ShellContent Route="r" ContentTemplate="{DataTemplate Page}"/> (route tables, target_page resolved in-file), plus Shell.Current.GoToAsync("//r") and GoToAsync(nameof(Page)) call sites (route normalized). Cross-file route->page resolution is partial. |
| Screen detection | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go` | ContentPage/Shell/TabbedPage/FlyoutPage/NavigationPage/MasterDetailPage subclass declarations emitted as SCOPE.UIComponent/screen_detection. |

### Platform

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Platform branching | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go` | DeviceInfo.Platform == DevicePlatform.X, Device.RuntimePlatform == Device.X (Xamarin), DeviceInfo.Idiom comparisons, and #if ANDROID/#if IOS/#if WINDOWS preprocessor directives detected. |

### Native Bridge

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Native module imports | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go` | [DllImport] P/Invoke declarations, DependencyService.Register<T>(), [assembly:Dependency(typeof(T))] platform registrations, and using Android./UIKit./WinRT. platform bridge imports detected. |

### Data Flow

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Branch conditions | 🟢 `partial` | — | — | `internal/custom/csharp/blazor_dataflow.go` | Device.RuntimePlatform/DeviceInfo.Platform platform conditionals + code if() branches detected via reBDFPlatformBranch/reBDFDeviceInfo/reBDFCodeIf |
| State management | 🟢 `partial` | — | — | `internal/custom/csharp/blazor_dataflow.go`<br>`internal/custom/csharp/mobile_maui_edges_test.go`<br>`internal/custom/csharp/mobile_platform.go` | MAUI BindableProperty.Create/SetValue + INotifyPropertyChanged callbacks (blazor_dataflow.go). MVVM view<->viewmodel wiring emits USES edges page->viewmodel:<VM>: BindingContext = new XViewModel() and DI-injected XViewModel ctor params on a ContentPage. CommunityToolkit.Mvvm [ObservableProperty]/[RelayCommand] mark a class as a ViewModel (viewmodel:<name> marker) and [RelayCommand] methods become command:<name> entities (mobile_platform.go). |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go`<br>`internal/extractors/csharp/csharp.go` | enum declarations emitted as SCOPE.Schema/enum_extraction via reMPEnum; the tree-sitter extractor also handles enums generically for all C# files. |
| Interface extraction | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go`<br>`internal/extractors/csharp/csharp.go` | interface declarations emitted as SCOPE.Component/interface_extraction via reMPInterface; the tree-sitter extractor also handles interfaces generically for all C# files. |
| Type alias extraction | — `not_applicable` | — | — | — | C# has only file-scoped using-aliases, not first-class type aliases |

### Lifecycle

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| State setter emission | 🟢 `partial` | — | — | `internal/custom/csharp/mobile_platform.go`<br>`internal/custom/csharp/mobile_platform_test.go` | CreateMauiApp/OnStart/OnSleep/OnResume/OnAppearing/OnDisappearing/OnNavigatedTo lifecycle method overrides, PropertyChanged?.Invoke(), OnPropertyChanged(), and SetProperty(ref _field) MVVM setter patterns detected. |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🟢 `partial` | — | — | `internal/extractors/cross/testmap/frameworks.go` | C# NUnit/xUnit/MSTest: [Fact]/[Theory]/[Test]/[TestMethod] attrs detected via csharpTestRE |

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | ✅ `full` | `2026-05-28` | — | `internal/graph/graph.go`<br>`internal/mcp/tools.go`<br>`internal/types/confidence.go` | — |
| Config consumption | ✅ `full` | `2026-06-02` | 3641 | `internal/extractor/config_key.go`<br>`internal/extractors/csharp/config_consumer.go`<br>`internal/extractors/csharp/config_consumer_test.go` | IConfiguration indexer/GetValue/GetConnectionString + Environment.GetEnvironmentVariable -> config:<key> (issue #3641) |
| Constant propagation | ✅ `full` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/csharp.go`<br>`internal/substrate/substrate.go` | — |
| DB effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_csharp.go` | — |
| Dead code detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_csharp.go` | — |
| Def use chain extraction | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/def_use_pass.go`<br>`internal/substrate/def_use_csharp.go` | — |
| Env fallback recognition | ✅ `full` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/csharp.go`<br>`internal/substrate/substrate.go` | — |
| Error flow | ✅ `full` | `2026-06-03` | 3628 | `internal/extractor/exception_flow.go`<br>`internal/extractors/csharp/exception_flow.go`<br>`internal/extractors/csharp/exception_flow_test.go` | throw new X / throw new pkg.X -> THROWS; catch (X ex) / catch (pkg.X) -> CATCHES; bare catch + throw;/throw e re-throw dropped (#3628) |
| Feature flag gating | 🟢 `partial` | `2026-06-03` | 3706 | `internal/engine/feature_flag_edges.go`<br>`internal/engine/feature_flag_edges_test.go`<br>`internal/engine/orm_queries.go` | flag-check call sites -> feature:<key> + GATED_BY (framework-agnostic C# engine pass, fires regardless of framework). .NET idioms attribute to the enclosing method: Microsoft.FeatureManagement _featureManager.IsEnabledAsync/IsEnabled("key") + [FeatureGate("key")] attribute, LaunchDarkly PascalCase BoolVariation/Variation, Unleash IsEnabled, OpenFeature GetBooleanValue. Honest-partial: dynamic keys + non-FeatureManager .IsEnabled miss (no literal / wrong receiver). |
| Fs effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_csharp.go` | — |
| HTTP effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_csharp.go` | — |
| Import resolution quality | 🟢 `partial` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/csharp.go`<br>`internal/substrate/substrate.go` | — |
| Module cycle detection | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/module_cycle_pass.go` | — |
| Mutation effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_csharp.go` | — |
| Pure function tagging | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/pure_function_pass.go` | — |
| Reachability analysis | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_csharp.go` | — |
| Request shape extraction | 🟢 `partial` | `2026-05-29` | [link](https://github.com/cajasmota/grafel/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_csharp.go` | — |
| Response shape extraction | 🟢 `partial` | `2026-05-29` | [link](https://github.com/cajasmota/grafel/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_csharp.go` | — |
| Sanitizer recognition | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_csharp.go` | — |
| Schema drift detection | 🟢 `partial` | `2026-05-29` | [link](https://github.com/cajasmota/grafel/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_csharp.go` | — |
| Taint sink detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_csharp.go` | — |
| Taint source detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_csharp.go` | — |
| Template pattern catalog | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/template_pattern_pass.go`<br>`internal/substrate/template_pattern_csharp.go` | — |
| Vulnerability finding | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_csharp.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.csharp.framework.net-maui ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
