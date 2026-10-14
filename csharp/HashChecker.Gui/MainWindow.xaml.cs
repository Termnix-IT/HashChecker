using System.IO;
using System.Security.Cryptography;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using Microsoft.Win32;
using HashChecker.Core;

namespace HashChecker.Gui;

public partial class MainWindow : Window
{
    private CancellationTokenSource? cancellation;
    private string SelectedAlgorithm => Algorithm.SelectedIndex == 1 ? "md5" : "sha256";

    public MainWindow()
    {
        InitializeComponent();
        Closing += (_, _) => cancellation?.Cancel();
        UpdateInputs();
    }

    private void Input_Changed(object sender, RoutedEventArgs e)
    {
        if (VerifyButton is null) return;
        ResultText.Clear();
        CopyButton.IsEnabled = false;
        StatusText.Text = "ファイルとハッシュ値を入力してください。";
        StatusText.Foreground = Brushes.Black;
        Progress.Value = 0;
        UpdateInputs();
    }

    private void UpdateInputs()
    {
        var busy = cancellation is not null;
        FilePath.IsEnabled = ExpectedHash.IsEnabled = Algorithm.IsEnabled = SelectButton.IsEnabled = !busy;
        CancelButton.IsEnabled = busy;
        VerifyButton.IsEnabled = !busy && !string.IsNullOrWhiteSpace(FilePath.Text) && !string.IsNullOrWhiteSpace(ExpectedHash.Text);
        HashHint.Text = SelectedAlgorithm == "md5" ? "32文字のMD5を貼り付けてください。" : "64文字のSHA256を貼り付けてください。";
    }

    private void Select_Click(object sender, RoutedEventArgs e)
    {
        var dialog = new OpenFileDialog { Title = "確認するファイルを選択", CheckFileExists = true, Multiselect = false };
        if (dialog.ShowDialog(this) == true) SelectFile(dialog.FileName);
    }

    private void SelectFile(string file)
    {
        if (cancellation is not null) return;
        if (!File.Exists(file))
        {
            StatusText.Text = "通常のファイルを1つ選択してください。";
            return;
        }
        FilePath.Text = file;
    }

    private void Window_DragOver(object sender, DragEventArgs e)
    {
        e.Effects = cancellation is null && e.Data.GetData(DataFormats.FileDrop) is string[] { Length: 1 } files
            && File.Exists(files[0]) ? DragDropEffects.Copy : DragDropEffects.None;
        e.Handled = true;
    }

    private void Window_Drop(object sender, DragEventArgs e)
    {
        if (cancellation is not null) { e.Handled = true; return; }
        if (e.Data.GetData(DataFormats.FileDrop) is string[] { Length: 1 } files) SelectFile(files[0]);
        else StatusText.Text = "ファイルを1つだけドラッグしてください。";
        e.Handled = true;
    }

    private async void Verify_Click(object sender, RoutedEventArgs e) => await VerifyAsync();

    private async Task VerifyAsync()
    {
        if (cancellation is not null) return;
        cancellation = new CancellationTokenSource();
        var source = cancellation;
        var algorithm = SelectedAlgorithm;
        ResultText.Clear();
        CopyButton.IsEnabled = false;
        Progress.Value = 0;
        StatusText.Text = "ハッシュ値を計算しています…";
        StatusText.Foreground = Brushes.Black;
        UpdateInputs();
        try
        {
            var progress = new Progress<double>(value =>
            {
                if (ReferenceEquals(cancellation, source)) Progress.Value = value;
            });
            var result = await Verifier.VerifyAsync(FilePath.Text.Trim(), ExpectedHash.Text, algorithm, progress, source.Token);
            Progress.Value = 100;
            StatusText.Text = result.Status == "match" ? "一致しました" : "一致しません";
            StatusText.Foreground = result.Status == "match" ? Brushes.DarkGreen : Brushes.DarkRed;
            ResultText.Text = $"ファイル: {result.Path}\n方式: {algorithm.ToUpperInvariant()}\n結果: {StatusText.Text}\n\n配布元のハッシュ値:\n{result.ExpectedHash}\n\n算出したハッシュ値:\n{result.ActualHash}";
            CopyButton.IsEnabled = true;
        }
        catch (OperationCanceledException)
        {
            StatusText.Text = "確認をキャンセルしました。";
            Progress.Value = 0;
        }
        catch (VerificationException ex) { StatusText.Text = ex.Message; StatusText.Foreground = Brushes.DarkRed; }
        catch (Exception) { StatusText.Text = "確認できませんでした。入力やファイルの状態を確認してください。"; }
        finally
        {
            cancellation = null;
            source.Dispose();
            UpdateInputs();
        }
    }

    private void Cancel_Click(object sender, RoutedEventArgs e) => cancellation?.Cancel();

    private void Copy_Click(object sender, RoutedEventArgs e)
    {
        try { Clipboard.SetText(ResultText.Text); }
        catch (System.Runtime.InteropServices.ExternalException)
        { StatusText.Text = "コピーできませんでした。もう一度お試しください。"; }
    }

    // Windows CI exercises real WPF controls and renders a reviewable result without UI automation dependencies.
    internal async Task RunSmokeAsync(string imagePath)
    {
        static void Check(bool condition) { if (!condition) throw new InvalidOperationException("GUI smoke failed."); }
        var folder = Path.Combine(Path.GetTempPath(), "hashchecker-gui-" + Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(folder);
        try
        {
            Check(!VerifyButton.IsEnabled && !CancelButton.IsEnabled && !CopyButton.IsEnabled);
            var file = Path.Combine(folder, "firmware.bin");
            var bytes = "HashChecker GUI sample"u8.ToArray();
            await File.WriteAllBytesAsync(file, bytes);
            SelectFile(folder);
            Check(FilePath.Text == "");
            SelectFile(file);
            Check(FilePath.Text == file && !VerifyButton.IsEnabled);
            ExpectedHash.Text = "invalid";
            await VerifyAsync();
            Check(!CopyButton.IsEnabled && CancelButton.IsEnabled == false && StatusText.Text.Contains("64"));
            ExpectedHash.Text = new string('0', 64);
            await VerifyAsync();
            Check(StatusText.Text == "一致しません" && CopyButton.IsEnabled);

            // Cancellation immediately after starting must restore all inputs and leave no old result.
            var largeFile = Path.Combine(folder, "large.bin");
            await File.WriteAllBytesAsync(largeFile, new byte[16 * 1024 * 1024]);
            SelectFile(largeFile);
            var task = VerifyAsync();
            Check(CancelButton.IsEnabled && !SelectButton.IsEnabled && !VerifyButton.IsEnabled);
            Cancel_Click(this, new RoutedEventArgs());
            await task;
            Check(StatusText.Text.Contains("キャンセル") && SelectButton.IsEnabled && !CopyButton.IsEnabled);

            SelectFile(file);
            Algorithm.SelectedIndex = 1;
            ExpectedHash.Text = Convert.ToHexStringLower(MD5.HashData(bytes));
            await VerifyAsync();
            Check(StatusText.Text == "一致しました" && HashHint.Text.Contains("32"));
            Algorithm.SelectedIndex = 0;
            ExpectedHash.Text = Convert.ToHexStringLower(SHA256.HashData(bytes));
            await VerifyAsync();
            Check(StatusText.Text == "一致しました" && CopyButton.IsEnabled && Progress.Value == 100);
            FilePath.TextChanged -= Input_Changed;
            FilePath.Text = "firmware.bin";
            FilePath.TextChanged += Input_Changed;
            UpdateLayout();
            var content = (FrameworkElement)Content;
            var bitmap = new RenderTargetBitmap((int)Math.Ceiling(content.ActualWidth + content.Margin.Left + content.Margin.Right),
                (int)Math.Ceiling(content.ActualHeight + content.Margin.Top + content.Margin.Bottom), 96, 96, PixelFormats.Pbgra32);
            bitmap.Render(this);
            var encoder = new PngBitmapEncoder();
            encoder.Frames.Add(BitmapFrame.Create(bitmap));
            using var output = File.Create(imagePath);
            encoder.Save(output);
        }
        finally { Directory.Delete(folder, true); }
    }
}
