// Numeric export from the exact qr dependency used by the pinned QrCodeView.
// No Flutter UI, application managers, network, or native plugins are started.
// Run with --packages=<reference-checkout>/.dart_tool/package_config.json.
import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:qr/qr.dart';

String fillUtf8(int size) {
  const prefix = 'https://example.invalid/';
  final rest = size - utf8.encode(prefix).length;
  return prefix + '界' * (rest ~/ 3) + 'a' * (rest % 3);
}

void main() {
  final inputs = <String, String>{
    'empty': '',
    'numeric': '0123456789012345678901234567890123456789',
    'alphanumeric': 'HTTPS://EXAMPLE.COM/SUB:12345',
    'url': 'https://example.invalid/subscription?token=AbC123&flag=%2f#keep',
    'verbatim': ' https://user:Pa%3Ass@example.invalid:8443/a%2Fb?token=AbC+123&z=%25#保留 ',
    'no-mixed-optimization': 'https://example.invalid/' + '0123456789' * 35,
    'latin1': 'https://example.invalid/café?clé=été',
    'unicode': 'https://例子.invalid/订阅?密钥=😀&x=%2F',
    for (final count in [14, 15, 180, 181, 213, 214, 1000, 1500, 2331, 2332])
      'bytes-$count': 'a' * count,
    'numeric-max': '1' * 5596,
    'numeric-over': '1' * 5597,
    'alphanumeric-max': 'A' * 3391,
    'alphanumeric-over': 'A' * 3392,
    'eci-max': fillUtf8(2330),
    'eci-over': fillUtf8(2331),
  };
  final cases = <Map<String, Object>>[];
  for (final entry in inputs.entries) {
    try {
      final code = QrCode(payload: QrPayload.fromString(entry.value));
      final image = QrImage(code);
      final bits = [
        for (var y = 0; y < image.moduleCount; y++)
          for (var x = 0; x < image.moduleCount; x++)
            image.isDark(y, x) ? 49 : 48,
      ];
      cases.add({
        'name': entry.key,
        'data': entry.value,
        'fits': true,
        'version': code.typeNumber,
        'modules': image.moduleCount,
        'mask': image.maskPattern,
        'sha256': sha256.convert(bits).toString(),
      });
    } on InputTooLongException {
      cases.add({'name': entry.key, 'data': entry.value, 'fits': false});
    }
  }
  final output = Platform.environment['FLCLASH_QR_PATH'];
  if (output == null) throw StateError('Set FLCLASH_QR_PATH');
  File(output).writeAsStringSync('${const JsonEncoder.withIndent('  ').convert({
    'upstream': '7fd417fb9a9a277c6a9339a8ac87f907df7c25f7',
    'qr': '4.0.0',
    'sdk': Platform.version,
    'errorCorrection': 'medium',
    'quietZone': 4,
    'cases': cases,
  })}\n');
}
