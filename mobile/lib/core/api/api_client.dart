import 'dart:typed_data';

import 'package:dio/dio.dart';

import '../error/failure.dart';
import 'api_response.dart';

/// The single HTTP surface features use. Unwraps the `{status,message,data}`
/// envelope and maps transport/envelope errors to typed [Failure]s.
class ApiClient {
  ApiClient(this._dio);
  final Dio _dio;

  Future<T> get<T>(
    String path, {
    Map<String, dynamic>? query,
    required T Function(Object? data) parse,
  }) {
    return _send(() => _dio.get<dynamic>(path, queryParameters: query), parse);
  }

  Future<T> post<T>(
    String path, {
    Object? body,
    required T Function(Object? data) parse,
  }) {
    return _send(() => _dio.post<dynamic>(path, data: body), parse);
  }

  Future<T> patch<T>(
    String path, {
    Object? body,
    required T Function(Object? data) parse,
  }) {
    return _send(() => _dio.patch<dynamic>(path, data: body), parse);
  }

  Future<T> delete<T>(
    String path, {
    required T Function(Object? data) parse,
  }) {
    return _send(() => _dio.delete<dynamic>(path), parse);
  }

  /// Multipart upload of one in-memory file under [field].
  Future<T> upload<T>(
    String path, {
    required String field,
    required Uint8List bytes,
    required String filename,
    Map<String, String>? fields,
    required T Function(Object? data) parse,
  }) {
    final form = FormData.fromMap({
      ...?fields,
      field: MultipartFile.fromBytes(bytes, filename: filename),
    });
    return _send(
      () => _dio.post<dynamic>(
        path,
        data: form,
        options: Options(contentType: 'multipart/form-data'),
      ),
      parse,
    );
  }

  /// Raw bytes from an image/thumb endpoint (auth header attached by interceptor).
  Future<Uint8List> getBytes(String path) async {
    try {
      final resp = await _dio.get<List<int>>(
        path,
        options: Options(responseType: ResponseType.bytes),
      );
      return Uint8List.fromList(resp.data ?? const []);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  Future<T> _send<T>(
    Future<Response<dynamic>> Function() call,
    T Function(Object? data) parse,
  ) async {
    try {
      final resp = await call();
      final body = resp.data;
      if (body is! Map<String, dynamic>) {
        throw const UnknownFailure('malformed response');
      }
      final env = ApiResponse<T>.fromJson(body, parse);
      if (!env.isOk) {
        throw ApiFailure(env.status, env.message);
      }
      return env.data as T;
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  Failure _mapDioError(DioException e) {
    if (e.response?.statusCode == 401) {
      return const AuthFailure();
    }
    return NetworkFailure(e.message ?? 'network error');
  }
}
