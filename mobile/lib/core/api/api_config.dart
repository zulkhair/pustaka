/// Backend connection settings. [baseUrl] always ends with `/api` and never has
/// a trailing slash.
class ApiConfig {
  const ApiConfig({
    required this.baseUrl,
    this.connectTimeout = const Duration(seconds: 15),
    this.receiveTimeout = const Duration(seconds: 60),
  });

  final String baseUrl;
  final Duration connectTimeout;
  final Duration receiveTimeout;

  /// Production/dev server (Caddy-fronted). Not yet deployed at time of writing.
  const ApiConfig.dev()
      : baseUrl = 'https://pustaka.example.com/api',
        connectTimeout = const Duration(seconds: 15),
        receiveTimeout = const Duration(seconds: 60);

  /// Android emulator → host loopback; the backend listens on 127.0.0.1:8002.
  const ApiConfig.local()
      : baseUrl = 'http://10.0.2.2:8002/api',
        connectTimeout = const Duration(seconds: 15),
        receiveTimeout = const Duration(seconds: 60);

  /// Resolves config at build time:
  ///  - `--dart-define=API_BASE_URL=https://host/api` overrides everything;
  ///  - else `--dart-define=USE_LOCAL=true` targets the local backend;
  ///  - else the default dev config.
  factory ApiConfig.fromEnvironment() {
    const override = String.fromEnvironment('API_BASE_URL');
    if (override.isNotEmpty) return ApiConfig(baseUrl: override);
    const useLocal = bool.fromEnvironment('USE_LOCAL');
    return useLocal ? const ApiConfig.local() : const ApiConfig.dev();
  }
}
