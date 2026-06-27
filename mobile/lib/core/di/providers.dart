import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/api_client.dart';
import '../api/api_config.dart';
import '../api/dio_factory.dart';
import '../auth/auth_controller.dart';
import '../auth/auth_interceptor.dart';
import '../auth/auth_service.dart';

final apiConfigProvider =
    Provider<ApiConfig>((ref) => ApiConfig.fromEnvironment());

/// A plain Dio with no auth interceptor — used to replay the original request
/// after a refresh (avoids re-entering the interceptor).
final plainDioProvider =
    Provider<Dio>((ref) => buildDio(ref.watch(apiConfigProvider)));

/// The authenticated Dio: attaches the bearer token and silently refreshes on
/// 401. The interceptor's callbacks `ref.read` the controller lazily at
/// call-time, which breaks the dio↔interceptor↔controller build-time cycle.
final dioProvider = Provider<Dio>((ref) {
  final interceptor = AuthInterceptor(
    storage: ref.watch(tokenStorageProvider),
    onRefresh: () => ref.read(authControllerProvider.notifier).refreshTokens(),
    onAuthLost: () => ref.read(authControllerProvider.notifier).markLoggedOut(),
    retryClient: ref.watch(plainDioProvider),
  );
  return buildDio(ref.watch(apiConfigProvider), interceptors: [interceptor]);
});

final apiClientProvider =
    Provider<ApiClient>((ref) => ApiClient(ref.watch(dioProvider)));

/// Root overrides applied by `main.dart`'s ProviderScope. Wires the real
/// AuthService (over the authenticated ApiClient) into the provider declared in
/// the auth layer.
final List<Override> rootOverrides = [
  authServiceProvider
      .overrideWith((ref) => AuthService(ref.watch(apiClientProvider))),
];
