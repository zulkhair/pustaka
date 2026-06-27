import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:pustaka/core/api/api_client.dart';

class _Adapter implements HttpClientAdapter {
  _Adapter(this.handler);
  final ResponseBody Function(RequestOptions o) handler;
  RequestOptions? last;

  @override
  void close({bool force = false}) {}

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    last = options;
    return handler(options);
  }
}

ResponseBody jsonResponse(Map<String, dynamic> m, {int status = 200}) {
  return ResponseBody.fromString(
    jsonEncode(m),
    status,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}

/// An ApiClient whose every request resolves to the given success [data] payload
/// wrapped in a `{status:0,...}` envelope.
ApiClient apiClientReturningData(Object? data) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test/api'));
  dio.httpClientAdapter = _Adapter(
      (_) => jsonResponse({'status': 0, 'message': 'ok', 'data': data}));
  return ApiClient(dio);
}

/// A 1x1 transparent PNG — a valid image so Image.memory decodes without error.
final Uint8List kTinyPng = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
);

/// An ApiClient whose getBytes resolves immediately to [bytes] (a valid image).
ApiClient apiClientReturningBytes(Uint8List bytes) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test/api'));
  dio.httpClientAdapter = _Adapter(
    (_) => ResponseBody.fromBytes(bytes, 200, headers: {
      Headers.contentTypeHeader: ['image/png'],
    }),
  );
  return ApiClient(dio);
}
