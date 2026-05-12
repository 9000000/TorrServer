const webpack = require('webpack')

module.exports = function override(config) {
    config.resolve.fallback = {
        ...config.resolve.fallback,
        path: require.resolve('path-browserify'),
        http: require.resolve('stream-http'),
        https: require.resolve('https-browserify'),
        stream: require.resolve('stream-browserify'),
        buffer: require.resolve('buffer/'),
        zlib: require.resolve('browserify-zlib'),
        assert: require.resolve('assert/'),
        util: require.resolve('util/'),
        url: require.resolve('url/'),
        querystring: require.resolve('querystring-es3'),
        events: require.resolve('events/'),
        string_decoder: require.resolve('string_decoder/'),
        constants: require.resolve('constants-browserify'),
        crypto: false,
        fs: false,
        os: false,
        net: false,
        tls: false,
        child_process: false,
        dgram: false,
        dns: false,
    }

    config.resolve.alias = {
        ...config.resolve.alias,
        'process/browser': require.resolve('process/browser.js'),
    }

    config.plugins = [
        ...config.plugins,
        new webpack.ProvidePlugin({
            process: 'process/browser.js',
            Buffer: ['buffer', 'Buffer'],
        }),
    ]

    // Ignore source-map warnings from node_modules
    config.ignoreWarnings = [/Failed to parse source map/]

    return config
}
