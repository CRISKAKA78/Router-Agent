using System.Globalization;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;

namespace ProbeTemplateGenerator.Services;

public abstract record ExpressionNode;
internal sealed record NumberExpression(double Value) : ExpressionNode;
internal sealed record TextExpression(string Value) : ExpressionNode;
internal sealed record ReferenceExpression(string Key) : ExpressionNode;
internal sealed record UnaryExpression(string Operator, ExpressionNode Operand) : ExpressionNode;
internal sealed record BinaryExpression(string Operator, ExpressionNode Left, ExpressionNode Right) : ExpressionNode;

/// <summary>The bounded expression language used by both preview and the BusyBox awk compiler.</summary>
public static partial class ExpressionParser
{
    private static readonly Dictionary<string, string> Aliases = new(StringComparer.Ordinal)
    {
        ["AND"] = "&&", ["and"] = "&&", ["OR"] = "||", ["or"] = "||", ["NOT"] = "!", ["not"] = "!",
        ["与"] = "&&", ["或"] = "||", ["非"] = "!"
    };
    private static readonly Dictionary<string, int> Precedence = new(StringComparer.Ordinal)
    {
        ["||"] = 1, ["&&"] = 2, ["=="] = 3, ["!="] = 3,
        ["<"] = 4, [">"] = 4, ["<="] = 4, [">="] = 4,
        ["+"] = 5, ["-"] = 5, ["*"] = 6, ["/"] = 6
    };

    [GeneratedRegex("^[+-]?(?:[0-9]+(?:\\.[0-9]*)?|\\.[0-9]+)(?:[eE][+-]?[0-9]+)?$")]
    private static partial Regex NumericPattern();
    [GeneratedRegex("^[a-z][a-z0-9_]{0,63}$")]
    internal static partial Regex KeyPattern();
    [GeneratedRegex("^(?:\"(?:[^\"\\\\\\r\\n]|\\\\[\"\\\\/bfnrt]|\\\\u[0-9a-fA-F]{4})*\")|^(?:[0-9]+(?:\\.[0-9]*)?|\\.[0-9]+)(?:[eE][+-]?[0-9]+)?|^[a-zA-Z_][a-zA-Z0-9_]*|^(?:&&|\\|\\||==|!=|<=|>=|[()+\\-*/!<>与或非])")]
    private static partial Regex TokenPattern();

    public static ExpressionNode Parse(string source, bool allowText = false)
    {
        if (string.IsNullOrWhiteSpace(source) || source.Length > 1024)
            throw new InvalidOperationException("公式不能为空，且最多 1024 个字符");
        var tokens = new List<string>();
        var rest = source.Trim();
        while (rest.Length > 0)
        {
            var match = TokenPattern().Match(rest);
            if (!match.Success) throw new InvalidOperationException($"公式包含不支持的字符：{rest[..Math.Min(12, rest.Length)]}");
            tokens.Add(Aliases.GetValueOrDefault(match.Value, match.Value));
            rest = rest[match.Length..].TrimStart();
            if (tokens.Count > 256) throw new InvalidOperationException("公式过长，最多 256 个符号");
        }
        var position = 0;
        string? Next() => position < tokens.Count ? tokens[position++] : null;
        ExpressionNode Primary(int depth)
        {
            if (depth > 32) throw new InvalidOperationException("公式嵌套最多 32 层");
            var token = Next();
            if (token is "!" or "+" or "-") return new UnaryExpression(token, Primary(depth + 1));
            if (token == "(")
            {
                var node = Expression(1, depth + 1);
                if (Next() != ")") throw new InvalidOperationException("公式缺少右括号");
                return node;
            }
            if (token is "true" or "false") return new NumberExpression(token == "true" ? 1 : 0);
            if (token?.StartsWith('"') == true)
            {
                if (!allowText) throw new InvalidOperationException("文本比较请使用“条件结果”获取方式");
                var value = JsonSerializer.Deserialize<string>(token)!;
                ValidateText(value.Length == 0 ? " " : value, 4096, "比较文本");
                return new TextExpression(value);
            }
            if (token is not null && NumericPattern().IsMatch(token)) return new NumberExpression(Number(token));
            if (token is not null && KeyPattern().IsMatch(token)) return new ReferenceExpression(token);
            throw new InvalidOperationException("公式缺少数值、属性标识或左括号");
        }
        ExpressionNode Expression(int min, int depth)
        {
            var left = Primary(depth);
            while (position < tokens.Count && Precedence.GetValueOrDefault(tokens[position]) >= min)
            {
                var op = tokens[position++];
                left = new BinaryExpression(op, left, Expression(Precedence[op] + 1, depth + 1));
            }
            return left;
        }
        var result = Expression(1, 0);
        if (position != tokens.Count) throw new InvalidOperationException($"公式中存在多余符号：{tokens[position]}");
        Validate(result);
        return result;
    }

    private static void Validate(ExpressionNode node)
    {
        switch (node)
        {
            case TextExpression: throw new InvalidOperationException("文本只支持与属性标识进行 == 或 != 比较");
            case BinaryExpression binary when IsTextComparison(binary):
                if (binary.Left is not (TextExpression or ReferenceExpression) || binary.Right is not (TextExpression or ReferenceExpression))
                    throw new InvalidOperationException("文本比较的另一侧须为属性标识或双引号文本");
                break;
            case UnaryExpression unary: Validate(unary.Operand); break;
            case BinaryExpression binary: Validate(binary.Left); Validate(binary.Right); break;
        }
    }

    internal static bool IsTextComparison(ExpressionNode node) => node is BinaryExpression { Operator: "==" or "!=" } binary &&
        (binary.Left is TextExpression || binary.Right is TextExpression);

    internal static IEnumerable<string> References(ExpressionNode node, bool numericOnly = false)
    {
        if (numericOnly && IsTextComparison(node)) yield break;
        switch (node)
        {
            case ReferenceExpression reference: yield return reference.Key; break;
            case UnaryExpression unary:
                foreach (var key in References(unary.Operand, numericOnly)) yield return key;
                break;
            case BinaryExpression binary:
                foreach (var key in References(binary.Left, numericOnly)) yield return key;
                foreach (var key in References(binary.Right, numericOnly)) yield return key;
                break;
        }
    }

    internal static double Evaluate(ExpressionNode node, IReadOnlyDictionary<string, double> values, IReadOnlyDictionary<string, string>? raw = null)
    {
        double Eval(ExpressionNode item) => Evaluate(item, values, raw);
        if (node is BinaryExpression text && IsTextComparison(text))
        {
            string Value(ExpressionNode item) => item is TextExpression literal ? literal.Value : raw![(item as ReferenceExpression)!.Key];
            var same = StringComparer.Ordinal.Equals(Value(text.Left), Value(text.Right));
            return (text.Operator == "==" ? same : !same) ? 1 : 0;
        }
        switch (node)
        {
            case NumberExpression number: return number.Value;
            case TextExpression: throw new InvalidOperationException("文本不能直接参与数值运算");
            case ReferenceExpression reference:
                return values.TryGetValue(reference.Key, out var value) ? value : throw new InvalidOperationException($"缺少属性：{reference.Key}");
            case UnaryExpression unary:
                var operand = Eval(unary.Operand);
                return unary.Operator switch { "!" => operand == 0 ? 1 : 0, "-" => -operand, _ => operand };
            case BinaryExpression binary:
                var a = Eval(binary.Left);
                if (binary.Operator == "&&") return a != 0 && Eval(binary.Right) != 0 ? 1 : 0;
                if (binary.Operator == "||") return a != 0 || Eval(binary.Right) != 0 ? 1 : 0;
                var b = Eval(binary.Right);
                return binary.Operator switch
                {
                    "+" => Finite(a + b), "-" => Finite(a - b), "*" => Finite(a * b),
                    "/" => b == 0 ? throw new InvalidOperationException("除数不能为 0") : Finite(a / b),
                    "==" => a == b ? 1 : 0, "!=" => a != b ? 1 : 0, "<" => a < b ? 1 : 0,
                    ">" => a > b ? 1 : 0, "<=" => a <= b ? 1 : 0, ">=" => a >= b ? 1 : 0,
                    _ => throw new InvalidOperationException("未知运算符")
                };
            default: throw new InvalidOperationException("未知公式节点");
        }
    }

    internal static string AwkExpression(ExpressionNode node, IReadOnlyDictionary<string, string> names)
    {
        if (node is BinaryExpression text && IsTextComparison(text))
        {
            string Value(ExpressionNode item) => item is TextExpression literal ? AwkString(literal.Value) : names[((ReferenceExpression)item).Key] + "s";
            // The prefix prevents awk's numeric-string coercion ("01" must differ from "1").
            return $"((\"x\" {Value(text.Left)}){text.Operator}(\"x\" {Value(text.Right)}))";
        }
        return node switch
        {
            NumberExpression number => NormalizeNumber(number.Value.ToString("R", CultureInfo.InvariantCulture)),
            ReferenceExpression reference => names[reference.Key],
            UnaryExpression unary => $"({unary.Operator}{AwkExpression(unary.Operand, names)})",
            BinaryExpression binary => Binary(binary),
            _ => throw new InvalidOperationException("文本不能直接参与数值运算")
        };
        string Binary(BinaryExpression binary)
        {
            var a = AwkExpression(binary.Left, names);
            var b = AwkExpression(binary.Right, names);
            return binary.Operator switch
            {
                "/" => $"div({a},{b})",
                "+" or "-" or "*" => $"finite({a}{binary.Operator}{b})",
                _ => $"({a}{binary.Operator}{b})"
            };
        }
    }

    internal static string AwkString(string value)
    {
        var output = new StringBuilder("\"");
        foreach (var c in value)
            output.Append(c switch
            {
                '"' => "\\\"", '\\' => "\\\\",
                < ' ' or '\x7f' => "\\" + Convert.ToString(c, 8).PadLeft(3, '0'),
                _ => c.ToString()
            });
        return output.Append('"').ToString();
    }

    internal static string Trim(string value) => value.Trim(' ', '\t', '\r', '\n');
    internal static int Bytes(string value) => Encoding.UTF8.GetByteCount(value);
    internal static void ValidateText(string value, int limit, string label)
    {
        if (string.IsNullOrEmpty(value) || Bytes(value) > limit || value.Contains('\0') || !ValidUnicode(value))
            throw new InvalidOperationException($"{label}不能为空，且不能超过 {limit} 字节或包含无效字符");
    }
    private static bool ValidUnicode(string value)
    {
        for (var i = 0; i < value.Length; i++)
            if (char.IsHighSurrogate(value[i]))
            {
                if (++i == value.Length || !char.IsLowSurrogate(value[i])) return false;
            }
            else if (char.IsLowSurrogate(value[i])) return false;
        return true;
    }
    internal static double Finite(double value) => double.IsFinite(value) && Math.Abs(value) <= 1e308
        ? value : throw new InvalidOperationException("运算结果超出有限数值范围");
    internal static double Number(string value)
    {
        var trimmed = Trim(value);
        if (trimmed == "true") return 1;
        if (trimmed == "false") return 0;
        if (!NumericPattern().IsMatch(trimmed)) throw new InvalidOperationException("运算输入必须为数字或 true / false");
        return Finite(double.Parse(trimmed, NumberStyles.Float, CultureInfo.InvariantCulture));
    }
    public static string FormatNumber(double value) => value == 0 ? "0" : NormalizeNumber(Finite(value).ToString("G12", CultureInfo.InvariantCulture));

    private static string NormalizeNumber(string value)
    {
        var e = value.IndexOfAny(['E', 'e']);
        if (e < 0) return value;
        var exponent = int.Parse(value[(e + 1)..], CultureInfo.InvariantCulture);
        var mantissa = value[..e];
        if (exponent is < -6 or >= 21) return mantissa + "e" + (exponent >= 0 ? "+" : "") + exponent.ToString(CultureInfo.InvariantCulture);
        var negative = mantissa.StartsWith('-');
        var digits = mantissa.TrimStart('-').Replace(".", "", StringComparison.Ordinal);
        var decimalPosition = exponent + 1;
        var expanded = decimalPosition <= 0 ? "0." + new string('0', -decimalPosition) + digits
            : decimalPosition >= digits.Length ? digits + new string('0', decimalPosition - digits.Length)
            : digits.Insert(decimalPosition, ".");
        return (negative ? "-" : "") + expanded;
    }
}
