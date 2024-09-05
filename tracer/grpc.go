/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file grpc.go
 * @package tracer
 * @author Dr.NP <np@herewe.tech>
 * @since 12/08/2023
 */

package tracer

// // ServerOption wrapper
// func NewGRPCClientInterceptor(tracer trace.Tracer) grpc.UnaryClientInterceptor {
// 	if tracer != nil {
// 		return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
// 			_, span := tracer.Start(ctx, method)
// 			defer span.End()

// 			err := invoker(ctx, method, req, reply, cc, opts...)
// 			if err != nil {
// 				span.RecordError(err)
// 			}

// 			return err
// 		}
// 	} else {
// 		return nil
// 	}
// }

// func NewGRPCServerInterceptor(tracer trace.Tracer) grpc.UnaryServerInterceptor {
// 	if tracer != nil {
// 		pg := b3.New()

// 		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
// 			reqHeader := make(http.Header)
// 			md, ok := metadata.FromIncomingContext(ctx)
// 			if ok {
// 				for k, v := range md {
// 					if len(v) > 0 {
// 						reqHeader.Add(k, v[0])
// 					}
// 				}
// 			}

// 			newCtx := pg.Extract(ctx, propagation.HeaderCarrier(reqHeader))
// 			spanedCtx, span := tracer.Start(newCtx, info.FullMethod)
// 			defer span.End()

// 			self := span.SpanContext()
// 			nmd := metadata.Pairs(
// 				"X-B3-Traceid", self.TraceID().String(),
// 				"X-B3-Spanid", self.SpanID().String(),
// 				"X-B3-Parentspanid", reqHeader.Get("X-B3-Spanid"),
// 				"X-B3-Sampled", reqHeader.Get("X-B3-Sampled"),
// 				"X-Request-ID", reqHeader.Get("X-Request-ID"),
// 			)
// 			savedCtx := metadata.NewOutgoingContext(spanedCtx, nmd)
// 			resp, err := handler(savedCtx, req)
// 			if err != nil {
// 				span.RecordError(err)
// 			}

// 			return resp, err
// 		}
// 	} else {
// 		return nil
// 	}
// }

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
