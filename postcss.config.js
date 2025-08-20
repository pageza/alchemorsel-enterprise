// PostCSS Configuration for Alchemorsel v3

const isProduction = process.env.NODE_ENV === 'production';

module.exports = {
  plugins: [
    require('postcss-preset-env')({
      stage: 1,
      features: {
        'nesting-rules': true,
        'custom-properties': true,
        'color-function': true
      }
    }),
    require('autoprefixer'),
    ...(isProduction ? [
      require('cssnano')({
        preset: ['default', {
          discardComments: { removeAll: true },
          normalizeWhitespace: true
        }]
      })
    ] : [])
  ]
};