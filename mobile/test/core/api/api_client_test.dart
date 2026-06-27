import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/api/api_client.dart';
import 'package:pustaka/core/error/failure.dart';

/// A programmable HttpClientAdapter so we exercise a real Dio + real envelope
/// decoding without a network or mocking Dio's generic methods.
class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this.handler);
  final Future<ResponseBody> Function(RequestOptions o) handler;
  RequestOptions? last;

  @override
  void close({bool force = false}) {}

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    last = options;
    return handler(options);
  }
}

ResponseBody _json(Map<String, dynamic> m, {int status = 200}) {
  return ResponseBody.fromString(
    jsonEncode(m),
    status,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}

ApiClient _clientWith(_FakeAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test/api'));
  dio.httpClientAdapter = adapter;
  return ApiClient(dio);
}

void main() {
  test('get decodes envelope and returns parsed data', () async {
    final client = _clientWith(_FakeAdapter(
      (_) async => _json({
        'status': 0,
        'message': 'ok',
        'data': {'x': 1}
      }),
    ));
    final out =
        await client.get<int>('/x', parse: (d) => (d! as Map)['x'] as int);
    expect(out, 1);
  });

  test('status 1 throws ApiFailure with message', () async {
    final client = _clientWith(_FakeAdapter(
      (_) async => _json({'status': 1, 'message': 'nope', 'data': null}),
    ));
    expect(
      () => client.get<Object?>('/x', parse: (d) => d),
      throwsA(isA<ApiFailure>().having((e) => e.message, 'message', 'nope')),
    );
  });

  test('connection error throws NetworkFailure', () async {
    final client = _clientWith(_FakeAdapter(
      (o) async => throw DioException(
          requestOptions: o, type: DioExceptionType.connectionError),
    ));
    expect(
      () => client.get<Object?>('/x', parse: (d) => d),
      throwsA(isA<NetworkFailure>()),
    );
  });

  test('upload builds a multipart form with the file part and extra fields',
      () async {
    final adapter = _FakeAdapter(
      (_) async => _json({
        'status': 0,
        'message': 'ok',
        'data': {'ok': true}
      }),
    );
    final client = _clientWith(adapter);
    await client.upload<Map<String, dynamic>>(
      '/documents/d1/pages',
      field: 'file',
      bytes: Uint8List.fromList([1, 2, 3]),
      filename: 'page.jpg',
      fields: {'note': 'hi'},
      parse: (d) => d! as Map<String, dynamic>,
    );
    final form = adapter.last!.data as FormData;
    expect(form.files.first.key, 'file');
    expect(form.files.first.value.filename, 'page.jpg');
    expect(form.fields.any((f) => f.key == 'note' && f.value == 'hi'), isTrue);
  });

  test('getBytes returns raw bytes', () async {
    final client = _clientWith(_FakeAdapter(
      (_) async => ResponseBody.fromBytes([9, 8, 7], 200,
          headers: {
            Headers.contentTypeHeader: ['image/jpeg'],
          }),
    ));
    final bytes = await client.getBytes('/documents/d1/pages/1/image');
    expect(bytes, Uint8List.fromList([9, 8, 7]));
  });
}
