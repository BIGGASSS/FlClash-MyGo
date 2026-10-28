// Export the exact unit-square Material shapes used by FlClash's dashboard.
// Run in the pinned Flutter checkout with both asset hooks disabled.
import 'dart:convert';
import 'dart:io';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_new_shapes/material_new_shapes.dart';

List<List<double>> curves(Iterable<Cubic> cubics) => [
  for (final c in cubics)
    [c.anchor0X, c.anchor0Y, c.control0X, c.control0Y,
     c.control1X, c.control1Y, c.anchor1X, c.anchor1Y],
];

void main() {
  test('export dashboard shapes', () async {
    final morph = Morph(MaterialShapes.circle, MaterialShapes.cookie9Sided);
    await File(Platform.environment['FLCLASH_SHAPES_PATH']!).writeAsString(jsonEncode({
      'clover4Leaf': curves(MaterialShapes.clover4Leaf.cubics),
      'cookie12Sided': curves(MaterialShapes.cookie12Sided.cubics),
      'start0': curves(morph.asCubics(0)),
      'start1': curves(morph.asCubics(1)),
    }));
  });
}
