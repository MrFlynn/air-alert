const path = require("path");
const autoprefixer = require("autoprefixer");
const cssnano = require("cssnano");
const MiniCssExtractPlugin = require("mini-css-extract-plugin");

module.exports = {
  output: {
    filename: "[name].bundle.js",
    chunkFilename: "[name].[chunkhash].chunk.js",
    path: path.resolve(__dirname, "dist"),
    publicPath: "/dist",
  },
  resolve: {
    extensions: [".css", ".scss"],
    alias: {
      "~": path.resolve(process.cwd(), 'src'),
    },
  },
  entry: {
    "styles": "./css/index.scss",
  },
  module: {
    rules: [
      {
        test: /\.scss$/,
        use: [
          MiniCssExtractPlugin.loader,
          {
            loader: "css-loader",
            options: {
              url: false,
              importLoaders: 1,
            },
          },
          {
            loader: "postcss-loader",
            options: {
              postcssOptions: {
                plugins: [
                  "autoprefixer", "cssnano"
                ]
              },
            },
          },
          {
            loader: "sass-loader",
          },
        ]
      },
    ]
  },
  plugins: [
    new MiniCssExtractPlugin({
      filename: "[name].css",
      chunkFilename: "[id].css",
    })
  ]
}