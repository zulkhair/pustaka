import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/api/api_config.dart';
import 'package:pustaka/core/api/dio_factory.dart';

void main() {
  test('dev baseUrl ends with /api and has no trailing slash', () {
    const cfg = ApiConfig.dev();
    expect(cfg.baseUrl.endsWith('/api'), isTrue);
    expect(cfg.baseUrl.endsWith('/'), isFalse);
  });

  test('buildDio applies baseUrl and Accept header', () {
    const cfg = ApiConfig.dev();
    final dio = buildDio(cfg);
    expect(dio.options.baseUrl, cfg.baseUrl);
    expect(dio.options.headers['Accept'], 'application/json');
  });
}
