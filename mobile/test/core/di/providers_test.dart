import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/api/api_client.dart';
import 'package:pustaka/core/auth/auth_controller.dart';
import 'package:pustaka/core/auth/auth_interceptor.dart';
import 'package:pustaka/core/di/providers.dart';

void main() {
  test('root provider graph builds without throwing', () {
    final c = ProviderContainer(overrides: rootOverrides);
    addTearDown(c.dispose);

    expect(c.read(apiClientProvider), isA<ApiClient>());
    expect(c.read(authControllerProvider.notifier), isNotNull);

    final dio = c.read(dioProvider);
    expect(dio.interceptors.whereType<AuthInterceptor>().isNotEmpty, isTrue);
  });
}
