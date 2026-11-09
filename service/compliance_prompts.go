package service

// Short imperative prompts for the cheap image model. Grid and fishnet match
// the tuned first-pass wording used by the video bridges; watercolor and
// colored-pencil stay face-local so the rest of the frame is unchanged.

const complianceFishnetPrompt = `Edit this photo. On EVERY human face, add a dense black fishnet veil that covers the whole face from the hairline to under the chin and from ear to ear.
Mesh: small diamond (rhombus) holes, about 30 diamonds across the face width; thick charcoal-black strands, each strand about as wide as the open hole; overall about 90% opaque. Multiple strands must cross each eye, each eyebrow, the nose and the lips so the eyes, eyebrows, nose and lips cannot be recognised. The mesh wraps the 3D shape of the face like stretched fabric, no eye or mouth openings.
Do NOT put mesh on hair, ears, neck, body, clothes or background. Keep the exact same framing, size, pose, expression, colours, lighting and background. No border, frame, padding, text or watermark.
If there is no human face, return the photo unchanged.`

const complianceGridPrompt = `Edit this photo. On EVERY human face, cover each eye region with a dense, nearly opaque medium-gray square grid.
Each eye patch spans from just above the eyebrow to below the lower eyelid, and from beyond the outer eye corner to the nose bridge; the two patches meet on the nose bridge. Grid: small square cells about 1/8 of the eye width, cells filled solid medium gray, thin slightly lighter gray seams, about 92% opaque. Pupils, irises, eyelids, eyelashes and eyebrows must be completely unreadable. Gray only, no coloured lines.
Do NOT change anything else: same face, mouth, nose, hair, body, clothes, background, framing, size and colours. No border, frame, padding, text or watermark.
If there is no human face, return the photo unchanged.`

const complianceWatercolorPrompt = `Conditional face-local watercolor portrait de-identification edit for Seedance image-to-video.

First inspect the image. Apply this edit only to real photographs or camera-like photorealistic people, including people in collages, character sheets, contact sheets, storyboards, reflections and small inset panels. If the image contains no photographic or photorealistic person, return it visually unchanged.

Primary visual target: preserve the real scene, body, clothing, products, props, text and background near-photographically, while making every qualifying FACE visibly watercolor-rendered. The watercolor artifact must remain visibly present at normal viewing distance, not only under pixel zoom, and must be confined to face and hair only.

For every photographic or photorealistic person:
- Preserve exactly the original canvas, aspect ratio, panel/layout structure, crop, composition, person count, body proportions, pose, gesture, action, gaze direction, expression category, head angle, hairstyle category and colour, clothing design/colour/material, accessories, hands, fingers, held products, props, readable product artwork, scene geometry, background objects, text, logos, lighting direction, colours and shadows.
- Preserve natural photographic depth, lens perspective, fabric texture, material reflections and environmental lighting outside the face-local treatment.
- Reconstruct the complete face and adjacent hair transitions with transparent watercolor washes, controlled pigment pooling, soft wet-on-dry edges and dry-brush edge variation. Use layered skin-tone washes and watercolor granulation that follow facial anatomy; retain readable natural eyes, eyelids, irises, pupils, brows, nose, lips, cheeks and jaw.
- Remove exact biometric identity through coordinated, visible reconstruction of eye shape/spacing, brows, nose, lips, cheeks, jaw and facial proportions. Preserve face direction, age category, expression, gaze and head pose, but produce a clearly non-identical fictional adult.
- Keep normal skin colour and realistic human anatomy. Watercolor is visible and unmistakable on the face, but it must not spread over clothing, products, text, panel dividers, scene geometry or the full frame.
- Preserve the original hairstyle, clothing folds, hands, products, props and all non-person pixels with realistic photographic material detail. Do not redraw the background into line art or paper.
- If the source is a multi-view sheet or collage, preserve every ORIGINAL panel, viewpoint, person instance and original panel divider line exactly. Do not invent a new character sheet, duplicated person, extra inset portrait, contact sheet or additional panel.
- Remove any pre-existing compliance residue that overlays a person: black eye grids, square cells, checkerboards, face mesh, censor bars, mosaics, black rectangles, tracking boxes or stickers are preprocessing artifacts, not source semantics. Reconstruct clean natural eyes in their place; every visible eye must have coherent iris, pupil, sclera, eyelids, lashes, brows and surrounding skin.
- Never add a grid, mask, mosaic, black rectangle, eye bar, censor strip, face box, sticker, text or new watermark over a person.
- No full-frame paper texture, global pencil hatching, painted background, illustrated clothing, wet wash over products, or watercolor bloom outside face and hair. The result is a real scene with a visible face-local watercolor compliance treatment, not a full-frame painting.

Output exactly one image with the original dimensions and layout.`

const compliancePencilPrompt = `Conditional museum-grade photorealistic colored-pencil portrait de-identification for Seedance image-to-video.

Inspect every panel and visible person. Apply only to photographic, camera-like photorealistic, painterly-realistic, or human-like 3D/CG faces. If no qualifying human face exists, return the input visually unchanged.

For every qualifying face:
- Preserve exactly the original canvas, aspect ratio, crop, composition, panel/layout structure, panel order, person count, pose, gesture, action, gaze direction, expression category, head angle, age category, hairstyle category and colour, body, clothing design/colour/material, accessories, hands, fingers, held products, props, readable product artwork, text, logos, background geometry, objects, lighting direction, colours and shadows.
- Reconstruct the complete face as a highly finished, photo-convincing colored-pencil portrait that is visibly colored-pencil-rendered at normal viewing distance. Authentic dry colored-pencil strokes and paper fibres remain unmistakable after resizing for video input; this is not a near-photographic no-op or a paper filter. Preserve realistic anatomy and material fidelity without reading as anime, beauty illustration, concept art or a 3D render.
- Build accurate skull and facial volume through anatomy-driven light and colour transitions across forehead, temples, eye sockets, cheekbones, nose bridge and wings, philtrum, mouth corners, jaw and neck. Use layered warm ochre, muted peach, red-brown, cool gray-violet and umber tones; preserve restrained natural asymmetry, subtle under-eye colour, fine expression lines and small complexion variations. No airbrushed, plastic, waxy, doll-like or cosmetically perfected skin.
- Render physiologically natural eyes on EVERY visible face instance, including tiny full-body, profile and inset views: warm-gray sclera rather than pure white, anatomically thick upper and lower lids, small tear ducts, lid-cast shadows, detailed but non-patterned irises, centered coherent pupils and tiny restrained moist catchlights. Keep the two eyes naturally non-identical. Every visible face must retain complete readable eye anatomy after editing, regardless of size; never blur, smear, erase, wash out or cover a small face. No oversized irises, beauty-contact look, heavy eyeliner, symmetric glass-bead highlights, rolled-back eyes, pupil-less eyes, milky film or hidden gaze.
- Construct hair in three levels: believable overall cranial volume, overlapping directional locks, then selected individual strands of varied thickness, length, brightness and curvature only at contours and lit crossings. Preserve natural roots and an irregular hairline. No ribbon hair, helmet hair, uniformly sharp strands or random white strokes. Apply the same density-and-direction logic to brows, lashes, beard and facial hair.
- Use fine contour-following pencil strokes, multi-directional cross-hatching and transparent layered colour. Pencil direction must follow facial planes and material form; never apply uniform diagonal hatching or a global digital pencil filter. Shadows are layered deep brown, warm black and charcoal gray rather than flat pure black.
- Use a restrained neutral gray-beige or muted gray-olive fibrous-paper character only where the source background permits it. Paper grain must be irregular and naturally less visible under dense pigment; never replace an existing scene, panel background, product artwork, text or logo with plain paper. No uniform digital noise, heavy paper grain, watercolor bloom, wet wash or paint drip.
- Remove exact biometric identity through coordinated but anatomically plausible reconstruction of eye spacing and shape, brows, nose, lips, cheeks, jaw and facial proportions. Preserve pose, head direction, expression, gaze and age category, but make the result a clearly non-identical fictional adult rather than an exact portrait copy. Identity abstraction must not reduce rendering precision.
- Preserve clothing, products, props, text, logos, body, hands and background faithfully. If the source is a collage, storyboard, contact sheet or multi-view board, preserve every ORIGINAL panel, divider, viewpoint and person instance. Never invent a character sheet, extra portrait inset, duplicated person, new panel, front/side/back view, plain-colour replacement background or redesigned costume.
- Remove all pre-existing compliance residue over a person, including black eye grids, square cells, checkerboards, fishnet, censor bars, mosaics, rectangles, tracking boxes, stickers, translucent films and construction lines. Reconstruct complete natural eyes and skin in their place. No grid, fishnet, bar, patch, milk-white film or rectangular boundary may remain.

Output exactly one image with the original dimensions, aspect ratio, layout and content order.`

func compliancePrompt(mode string) string {
	switch mode {
	case "grid":
		return complianceGridPrompt
	case "watercolor":
		return complianceWatercolorPrompt
	case "colored-pencil":
		return compliancePencilPrompt
	default:
		return complianceFishnetPrompt
	}
}
