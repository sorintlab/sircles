const webpack = require('webpack')
const path = require('path')
const distPath = path.resolve(__dirname, 'dist')
const nodeModulesPath = path.resolve(__dirname, 'node_modules')
const ExtractTextPlugin = require('extract-text-webpack-plugin')

module.exports = function (env) {
  const isProd = env && env.prod

  console.log('production mode:', isProd ? 'true' : 'false')

  const plugins = [
    new ExtractTextPlugin('styles.css'),
    new webpack.optimize.CommonsChunkPlugin({
      name: 'vendor',
      minChunks: Infinity,
      filename: 'vendor.bundle.js'
    }),
    new webpack.NoEmitOnErrorsPlugin()
  ]

  if (isProd) {
    plugins.push(
    new webpack.DefinePlugin({
      'process.env': {
        'NODE_ENV': JSON.stringify('production')
      }
    }),
      new webpack.optimize.UglifyJsPlugin({
        compress: {
          // suppresses warnings, usually from module minification
          warnings: false
        }
      })
    )
  } else {
    plugins.push(
      new webpack.HotModuleReplacementPlugin()
    )
  }

  return {
    // Entry points to the project
    entry: path.join(__dirname, '/src/app/app.js'),
    output: {
      path: distPath, // Path of output file
      filename: 'app.js'
    },
    externals: {
      'config': 'CONFIG'
    },
    // Server Configuration options
    devServer: {
      contentBase: './src/www',
      hot: !isProd,
      inline: true,
      port: 3000, // Port Number
      host: 'localhost', // Change to '0.0.0.0' for external facing server
      historyApiFallback: true
    },
    devtool: isProd ? 'source-map' : 'eval',
    plugins: plugins,
    module: {
      rules: [
        {
          // React-hot loader and
          test: /\.js$/, // All .js files
          loaders: ['react-hot-loader', 'babel-loader'], // react-hot is like browser sync and babel loads jsx and es6-7
          exclude: [nodeModulesPath]
        },
        {
          test: /\.html$/,
          loader: 'file-loader',
          query: {
            name: '[name].[ext]'
          },
          exclude: [nodeModulesPath]
        },
        {
          test: /\.css$/,
          use: ExtractTextPlugin.extract({
            fallback: 'style-loader',
            use: 'css-loader'
          }),
          include: [
            path.join(__dirname, 'src'),
            path.join(__dirname, 'semantic'),
            path.join(nodeModulesPath, 'react-image-crop/dist/'),
            path.join(nodeModulesPath, 'react-day-picker/lib/')
          ]
        },
        {
          test: /\.(png|jpg|gif|svg|eot|ttf|woff|woff2)$/,
          loader: 'file-loader',
          options: {
            limit: 10000
          },
          exclude: [nodeModulesPath]
        },
        {
          test: /\.(graphql|gql)$/,
          loader: 'graphql-tag/loader',
          exclude: [nodeModulesPath]
        }
      ]
    }
  }
}
