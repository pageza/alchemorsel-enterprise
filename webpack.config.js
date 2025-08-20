// Alchemorsel v3 Webpack Configuration
// Hot reload asset pipeline for development

const path = require('path');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');

const isProduction = process.env.NODE_ENV === 'production';

module.exports = {
  mode: isProduction ? 'production' : 'development',
  
  entry: {
    main: './src/js/app.js',
    admin: './src/js/admin.js',
    htmx: './src/js/htmx-extensions.js'
  },
  
  output: {
    path: path.resolve(__dirname, 'internal/infrastructure/http/server/static/js'),
    filename: isProduction ? '[name].[contenthash].js' : '[name].js',
    clean: true,
    publicPath: '/static/js/'
  },
  
  module: {
    rules: [
      // JavaScript
      {
        test: /\.js$/,
        exclude: /node_modules/,
        use: {
          loader: 'babel-loader',
          options: {
            presets: ['@babel/preset-env'],
            cacheDirectory: true
          }
        }
      },
      
      // SCSS/CSS
      {
        test: /\.s?css$/,
        use: [
          isProduction ? MiniCssExtractPlugin.loader : 'style-loader',
          'css-loader',
          {
            loader: 'postcss-loader',
            options: {
              postcssOptions: {
                plugins: [
                  ['autoprefixer'],
                  ...(isProduction ? [['cssnano', { preset: 'default' }]] : [])
                ]
              }
            }
          },
          'sass-loader'
        ]
      }
    ]
  },
  
  plugins: [
    ...(isProduction ? [
      new MiniCssExtractPlugin({
        filename: '../css/[name].[contenthash].css'
      })
    ] : [])
  ],
  
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
      '@js': path.resolve(__dirname, 'src/js'),
      '@scss': path.resolve(__dirname, 'src/scss')
    }
  },
  
  devtool: isProduction ? 'source-map' : 'eval-source-map',
  
  watchOptions: {
    ignored: /node_modules/,
    poll: 1000,
    aggregateTimeout: 300
  },
  
  optimization: {
    splitChunks: {
      chunks: 'all',
      cacheGroups: {
        vendor: {
          test: /[\\/]node_modules[\\/]/,
          name: 'vendors',
          chunks: 'all'
        },
        htmx: {
          test: /htmx/,
          name: 'htmx',
          chunks: 'all'
        }
      }
    }
  },
  
  stats: {
    errorDetails: true,
    children: false,
    modules: false,
    entrypoints: false
  }
};