import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/auth/auth_interceptor.dart';

import '../../support/fake_token_storage.dart';

class _Adapter implements HttpClientAdapter {
  int needauthHits = 0;
  String? lastAuthHeader;

  @override
  void close({bool force = false}) {}

  @override
  Future<ResponseBody> fetch(
    RequestOptions o,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastAuthHeader = o.headers['Authorization'] as String?;
    if (o.path == '/needauth' && o.extra['retried'] != true) {
      needauthHits++;
      return _body({'status': 1, 'message': 'unauthorized', 'data': null}, 401);
    }
    return _body({'status': 0, 'message': 'ok', 'data': null}, 200);
  }
}

ResponseBody _body(Map<String, dynamic> m, int status) {
  return ResponseBody.fromString(
    jsonEncode(m),
    status,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}

({Dio dio, _Adapter adapter}) _build({
  required FakeTokenStorage storage,
  required Future<String?> Function() onRefresh,
  required void Function() onAuthLost,
}) {
  final adapter = _Adapter();
  final retry = Dio(BaseOptions(baseUrl: 'http://t'))..httpClientAdapter = adapter;
  final dio = Dio(BaseOptions(baseUrl: 'http://t'))..httpClientAdapter = adapter;
  dio.interceptors.add(AuthInterceptor(
    storage: storage,
    onRefresh: onRefresh,
    onAuthLost: onAuthLost,
    retryClient: retry,
  ));
  return (dio: dio, adapter: adapter);
}

void main() {
  test('onRequest attaches bearer token from storage', () async {
    final storage = FakeTokenStorage();
    await storage.write(access: 'access1', refresh: 'r1');
    final b = _build(storage: storage, onRefresh: () async => 'new', onAuthLost: () {});
    await b.dio.get<dynamic>('/ok');
    expect(b.adapter.lastAuthHeader, 'Bearer access1');
  });

  test('401 triggers refresh + retry once, resolves with success', () async {
    final storage = FakeTokenStorage();
    await storage.write(access: 'old', refresh: 'r1');
    var refreshCalls = 0;
    final b = _build(
      storage: storage,
      onRefresh: () async {
        refreshCalls++;
        return 'newtok';
      },
      onAuthLost: () {},
    );
    final resp = await b.dio.get<dynamic>('/needauth');
    expect(resp.statusCode, 200);
    expect(refreshCalls, 1);
  });

  test('failed refresh calls onAuthLost and throws', () async {
    final storage = FakeTokenStorage();
    await storage.write(access: 'old', refresh: 'r1');
    var lost = false;
    final b = _build(
      storage: storage,
      onRefresh: () async => null,
      onAuthLost: () => lost = true,
    );
    await expectLater(b.dio.get<dynamic>('/needauth'), throwsA(isA<DioException>()));
    expect(lost, isTrue);
  });

  test('two concurrent 401s refresh exactly once', () async {
    final storage = FakeTokenStorage();
    await storage.write(access: 'old', refresh: 'r1');
    var refreshCalls = 0;
    final b = _build(
      storage: storage,
      onRefresh: () async {
        refreshCalls++;
        await Future<void>.delayed(const Duration(milliseconds: 10));
        return 'newtok';
      },
      onAuthLost: () {},
    );
    await Future.wait([b.dio.get<dynamic>('/needauth'), b.dio.get<dynamic>('/needauth')]);
    expect(refreshCalls, 1);
  });
}
