// Run in the pinned Flutter checkout; see README.md.
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:fl_clash/ui/icons/icons.dart';

final glyphs = <String, Glyph>{
  'dashboard': AppGlyphs.dashboard,
  'proxies': AppGlyphs.proxies,
  'profiles': AppGlyphs.profiles,
  'requests': AppGlyphs.requests,
  'clock': AppGlyphs.clock,
  'connections': AppGlyphs.connections,
  'dns': AppGlyphs.dns,
  'resources': AppGlyphs.resources,
  'logs': AppGlyphs.logs,
  'tools': AppGlyphs.tools,
  'target': AppGlyphs.target,
  'arrowBack': AppGlyphs.arrowBack,
  'chevronBack': AppGlyphs.chevronBack,
  'chevronForward': AppGlyphs.chevronForward,
  'chevronDown': AppGlyphs.chevronDown,
  'chevronUp': AppGlyphs.chevronUp,
  'picker': AppGlyphs.picker,
  'caretDown': AppGlyphs.caretDown,
  'arrowUp': AppGlyphs.arrowUp,
  'arrowDown': AppGlyphs.arrowDown,
  'openExternal': AppGlyphs.openExternal,
  'undo': AppGlyphs.undo,
  'redo': AppGlyphs.redo,
  'sync': AppGlyphs.sync,
  'refresh': AppGlyphs.refresh,
  'reset': AppGlyphs.reset,
  'export': AppGlyphs.export,
  'upload': AppGlyphs.upload,
  'add': AppGlyphs.add,
  'remove': AppGlyphs.remove,
  'addCircle': AppGlyphs.addCircle,
  'removeCircle': AppGlyphs.removeCircle,
  'close': AppGlyphs.close,
  'clearField': AppGlyphs.clearField,
  'check': AppGlyphs.check,
  'checkDouble': AppGlyphs.checkDouble,
  'checkCircle': AppGlyphs.checkCircle,
  'block': AppGlyphs.block,
  'info': AppGlyphs.info,
  'warning': AppGlyphs.warning,
  'error': AppGlyphs.error,
  'more': AppGlyphs.more,
  'moreCircle': AppGlyphs.moreCircle,
  'dragHandle': AppGlyphs.dragHandle,
  'sort': AppGlyphs.sort,
  'filter': AppGlyphs.filter,
  'textShort': AppGlyphs.textShort,
  'wrap': AppGlyphs.wrap,
  'listAdd': AppGlyphs.listAdd,
  'edit': AppGlyphs.edit,
  'compose': AppGlyphs.compose,
  'delete': AppGlyphs.delete,
  'clearAll': AppGlyphs.clearAll,
  'copy': AppGlyphs.copy,
  'cut': AppGlyphs.cut,
  'paste': AppGlyphs.paste,
  'selectAll': AppGlyphs.selectAll,
  'deselect': AppGlyphs.deselect,
  'search': AppGlyphs.search,
  'matchCase': AppGlyphs.matchCase,
  'regex': AppGlyphs.regex,
  'importFile': AppGlyphs.importFile,
  'cloudDownload': AppGlyphs.cloudDownload,
  'cloudSync': AppGlyphs.cloudSync,
  'link': AppGlyphs.link,
  'send': AppGlyphs.send,
  'rules': AppGlyphs.rules,
  'rulePresets': AppGlyphs.rulePresets,
  'code': AppGlyphs.code,
  'settings': AppGlyphs.settings,
  'sliders': AppGlyphs.sliders,
  'wrench': AppGlyphs.wrench,
  'toggle': AppGlyphs.toggle,
  'toggleOn': AppGlyphs.toggleOn,
  'pin': AppGlyphs.pin,
  'eye': AppGlyphs.eye,
  'eyeOff': AppGlyphs.eyeOff,
  'lock': AppGlyphs.lock,
  'lockOpen': AppGlyphs.lockOpen,
  'key': AppGlyphs.key,
  'password': AppGlyphs.password,
  'account': AppGlyphs.account,
  'sparkle': AppGlyphs.sparkle,
  'bolt': AppGlyphs.bolt,
  'locate': AppGlyphs.locate,
  'star': AppGlyphs.star,
  'puzzle': AppGlyphs.puzzle,
  'customize': AppGlyphs.customize,
  'layers': AppGlyphs.layers,
  'sun': AppGlyphs.sun,
  'pureBlack': AppGlyphs.pureBlack,
  'palette': AppGlyphs.palette,
  'paintbrush': AppGlyphs.paintbrush,
  'eyedropper': AppGlyphs.eyedropper,
  'motion': AppGlyphs.motion,
  'blur': AppGlyphs.blur,
  'liquidGlass': AppGlyphs.liquidGlass,
  'font': AppGlyphs.font,
  'textSize': AppGlyphs.textSize,
  'language': AppGlyphs.language,
  'keyboard': AppGlyphs.keyboard,
  'photos': AppGlyphs.photos,
  'camera': AppGlyphs.camera,
  'qrCode': AppGlyphs.qrCode,
  'torch': AppGlyphs.torch,
  'torchOff': AppGlyphs.torchOff,
  'torchAuto': AppGlyphs.torchAuto,
  'vpn': AppGlyphs.vpn,
  'status': AppGlyphs.status,
  'verified': AppGlyphs.verified,
  'swap': AppGlyphs.swap,
  'shuffle': AppGlyphs.shuffle,
  'split': AppGlyphs.split,
  'speed': AppGlyphs.speed,
  'dataUsage': AppGlyphs.dataUsage,
  'memory': AppGlyphs.memory,
  'cpu': AppGlyphs.cpu,
  'devices': AppGlyphs.devices,
  'broom': AppGlyphs.broom,
  'wifi': AppGlyphs.wifi,
  'wifiOff': AppGlyphs.wifiOff,
  'gavel': AppGlyphs.gavel,
  'appsList': AppGlyphs.appsList,
  'iconTile': AppGlyphs.iconTile,
  'sidebar': AppGlyphs.sidebar(1),
  'sidebarCompact': AppGlyphs.sidebar(0),
  'play': AppGlyphs.playPause(0),
  'pause': AppGlyphs.playPause(1),
};

String number(double value) => value.toStringAsFixed(5);

String sampledPath(Path path) {
  final result = StringBuffer();
  for (final metric in path.computeMetrics()) {
    final points = <Offset>[];
    final count = math.max(2, (metric.length / 0.08).ceil());
    for (var i = 0; i <= count; i++) {
      final tangent = metric.getTangentForOffset(metric.length * i / count);
      if (tangent != null) points.add(tangent.position);
    }
    final kept = simplify(points, 0.001);
    for (var i = 0; i < kept.length; i++) {
      result.write(
        '${i == 0 ? 'M' : 'L'}${number(kept[i].dx)} ${number(kept[i].dy)}',
      );
    }
    if (metric.isClosed) result.write('Z');
  }
  return result.toString();
}

List<Offset> simplify(List<Offset> points, double tolerance) {
  if (points.length <= 2) return points;
  final delta = points.last - points.first;
  final squared = delta.dx * delta.dx + delta.dy * delta.dy;
  var furthest = 0.0;
  var split = 0;
  for (var i = 1; i < points.length - 1; i++) {
    final from = points[i] - points.first;
    final t = squared == 0
        ? 0.0
        : ((from.dx * delta.dx + from.dy * delta.dy) / squared).clamp(0.0, 1.0);
    final distance = (points[i] - (points.first + delta * t)).distance;
    if (distance > furthest) {
      furthest = distance;
      split = i;
    }
  }
  if (furthest <= tolerance) return [points.first, points.last];
  return [
    ...simplify(points.sublist(0, split + 1), tolerance)..removeLast(),
    ...simplify(points.sublist(split), tolerance),
  ];
}

String geometry(GlyphShape shape) {
  if (shape is GlyphCircle) {
    return '<circle cx="${shape.x}" cy="${shape.y}" r="${shape.radius}"/>';
  }
  if (shape is GlyphOval) {
    return '<ellipse cx="${(shape.left + shape.right) / 2}" cy="${(shape.top + shape.bottom) / 2}" rx="${(shape.right - shape.left) / 2}" ry="${(shape.bottom - shape.top) / 2}"/>';
  }
  if (shape is GlyphPath) return '<path d="${shape.data}"/>';
  if (shape is GlyphPolyline) {
    final v = shape.vertices;
    (Offset, Offset) corner(int i) {
      final vertex = v[i].offset;
      final previous = v[(i - 1 + v.length) % v.length].offset - vertex;
      final next = v[(i + 1) % v.length].offset - vertex;
      return (
        vertex +
            previous /
                previous.distance *
                math.min(v[i].radius, previous.distance / 2),
        vertex +
            next / next.distance * math.min(v[i].radius, next.distance / 2),
      );
    }

    final first = shape.closed ? corner(0).$2 : v.first.offset;
    final path = StringBuffer('M${number(first.dx)} ${number(first.dy)}');
    for (var i = 1; i <= (shape.closed ? v.length : v.length - 1); i++) {
      final vertex = v[i % v.length];
      if (!shape.closed && i == v.length - 1) {
        path.write('L${vertex.x} ${vertex.y}');
      } else {
        final (enter, exit) = corner(i % v.length);
        path.write(
          'L${number(enter.dx)} ${number(enter.dy)}Q${vertex.x} ${vertex.y} ${number(exit.dx)} ${number(exit.dy)}',
        );
      }
    }
    if (shape.closed) path.write('Z');
    return '<path d="$path"/>';
  }
  if (shape is GlyphLink) {
    final delta = shape.to - shape.from;
    final inset = delta / delta.distance * (shape.radius + Glyph.strokeWidth);
    final start = shape.from + inset;
    final end = shape.to - inset;
    return '<path d="M${number(start.dx)} ${number(start.dy)}L${number(end.dx)} ${number(end.dy)}"/>';
  }
  if (shape is GlyphArc) {
    final x = shape.x + shape.radius * math.cos(shape.start);
    final y = shape.y + shape.radius * math.sin(shape.start);
    final endX = shape.x + shape.radius * math.cos(shape.start + shape.sweep);
    final endY = shape.y + shape.radius * math.sin(shape.start + shape.sweep);
    return '<path d="M${number(x)} ${number(y)}A${shape.radius} ${shape.radius} 0 ${shape.sweep.abs() > math.pi ? 1 : 0} ${shape.sweep > 0 ? 1 : 0} ${number(endX)} ${number(endY)}"/>';
  }
  return '<path d="${sampledPath(shape.path)}"/>';
}

String paint(GlyphShape shape, {bool filled = false, bool cut = false}) {
  final color = cut ? 'black' : 'currentColor';
  final fill = shape.solid || (filled && shape.role == GlyphRole.body);
  final stroke = shape.solid ? 'none' : color;
  final width = filled && shape.role == GlyphRole.line
      ? 2.2
      : Glyph.strokeWidth;
  var transform = '';
  if (filled && shape.solid && shape.role == GlyphRole.line) {
    final center = shape.path.getBounds().center;
    transform =
        ' transform="translate(${center.dx} ${center.dy}) scale(${2.2 / Glyph.strokeWidth}) translate(${-center.dx} ${-center.dy})"';
  }
  return '<g fill="${fill ? color : 'none'}" stroke="$stroke" stroke-width="$width"$transform>${geometry(shape)}</g>';
}

String svg(Glyph glyph, bool filled) {
  final output = StringBuffer(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-linecap="round" stroke-linejoin="round">',
  );
  if (filled && glyph.shapes.any((shape) => shape.role == GlyphRole.detail)) {
    output.write(
      '<defs><mask id="details" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24" mask-type="luminance"><rect width="24" height="24" fill="white"/>',
    );
    for (final shape in glyph.shapes.where(
      (shape) => shape.role == GlyphRole.detail,
    )) {
      output.write(paint(shape, cut: true));
    }
    output.write('</mask></defs><g mask="url(#details)">');
    for (final shape in glyph.shapes.where(
      (shape) => shape.role == GlyphRole.body,
    )) {
      output.write(
        '<g fill="currentColor" stroke="none">${geometry(shape)}</g>',
      );
    }
    output.write('</g>');
    for (final shape in glyph.shapes.where(
      (shape) => shape.role == GlyphRole.body,
    )) {
      output.write(paint(shape));
    }
    for (final shape in glyph.shapes.where(
      (shape) => shape.role == GlyphRole.line,
    )) {
      output.write(paint(shape, filled: true));
    }
  } else {
    for (final shape in glyph.shapes) {
      output.write(paint(shape, filled: filled));
    }
  }
  output.write('</svg>');
  return output.toString();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('export original caption vectors', () async {
    String path(Path p) => '<path d="${sampledPath(p)}"/>';
    final box = Rect.fromLTRB(.5,.5,9.5,9.5);
    final front = Rect.fromLTRB(.5,2.5,7.5,9.5);
    String outline(Rect r) => path(Path()..addRSuperellipse(RSuperellipse.fromRectAndRadius(r,const Radius.circular(1))));
    final bodies = {
      'minimize': '<path d="M0 5.5L10 5.5"/>',
      'maximize': outline(box),
      'restore': '${outline(front)}<defs><mask id="front" maskUnits="userSpaceOnUse" x="0" y="0" width="10" height="10"><rect width="10" height="10" fill="white" stroke="none"/><rect x="0" y="2" width="8" height="8" fill="black" stroke="none"/></mask></defs><g mask="url(#front)">${outline(front.shift(const Offset(2,-2)))}</g>',
      'close': '<path stroke-linecap="round" d="M.5 .5L9.5 9.5M9.5 .5L.5 9.5"/>',
    };
    await File(Platform.environment['FLCLASH_CAPTION_PATH']!).writeAsString(jsonEncode({
      for (final entry in bodies.entries) entry.key: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="butt" stroke-linejoin="round">${entry.value}</svg>',
    }));
  });
  test('export original glyph vectors', () async {
    final output = <String, Object>{};
    for (final MapEntry(key: name, value: glyph) in glyphs.entries) {
      output[name] = {
        'outline': svg(glyph, false),
        'filled': svg(glyph, true),
        'rtl': glyph.matchTextDirection,
      };
    }
    final target = File(Platform.environment['FLCLASH_GLYPHS_PATH']!);
    await target.parent.create(recursive: true);
    await target.writeAsString(jsonEncode(output));
  });
}
