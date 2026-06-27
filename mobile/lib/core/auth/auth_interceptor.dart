import 'package:dio/dio.dart';

import 'token_storage.dart';

/// Attaches the bearer token and performs a single-flight silent refresh on 401.
/// Decoupled from the auth controller via callbacks to avoid an import cycle.
class AuthInterceptor extends Interceptor {
  AuthInterceptor({
    required this.storage,
    required this.onRefresh,
    required this.onAuthLost,
    required this.retryClient,
  });

  final TokenStorage storage;
  final Future<String?> Function() onRefresh;
  final void Function() onAuthLost;

  /// A plain Dio (no auth interceptor) used to replay the original request.
  final Dio retryClient;

  Future<String?>? _refreshing;

  @override
  Future<void> onRequest(RequestOptions options, RequestInterceptorHandler handler) async {
    final tokens = await storage.read();
    final access = tokens.access;
    if (access != null && access.isNotEmpty) {
      options.headers['Authorization'] = 'Bearer $access';
    }
    handler.next(options);
  }

  @override
  Future<void> onError(DioException err, ErrorInterceptorHandler handler) async {
    final opts = err.requestOptions;
    final is401 = err.response?.statusCode == 401;
    final isRefreshCall = opts.path.contains('/auth/refresh');
    final alreadyRetried = opts.extra['retried'] == true;

    if (!is401 || isRefreshCall || alreadyRetried) {
      handler.next(err);
      return;
    }

    // Single-flight: concurrent 401s share one refresh.
    final future = _refreshing ??= onRefresh();
    String? newAccess;
    try {
      newAccess = await future;
    } finally {
      if (identical(_refreshing, future)) {
        _refreshing = null;
      }
    }

    if (newAccess == null) {
      onAuthLost();
      handler.next(err);
      return;
    }

    opts.extra['retried'] = true;
    opts.headers['Authorization'] = 'Bearer $newAccess';
    try {
      final resp = await retryClient.fetch<dynamic>(opts);
      handler.resolve(resp);
    } on DioException catch (e) {
      handler.next(e);
    }
  }
}
