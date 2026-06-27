import 'package:dio/dio.dart';

import 'api_config.dart';

/// Builds a configured [Dio] for [cfg], attaching any [interceptors] in order.
Dio buildDio(ApiConfig cfg, {List<Interceptor> interceptors = const []}) {
  final dio = Dio(
    BaseOptions(
      baseUrl: cfg.baseUrl,
      connectTimeout: cfg.connectTimeout,
      receiveTimeout: cfg.receiveTimeout,
      headers: const {'Accept': 'application/json'},
    ),
  );
  dio.interceptors.addAll(interceptors);
  return dio;
}
