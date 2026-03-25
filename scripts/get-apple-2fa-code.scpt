-- Usage:
--   osascript /absolute/path/to/get-apple-2fa-code.scpt
--   osascript /absolute/path/to/get-apple-2fa-code.scpt 90
--
-- This script polls the macOS FollowUpUI accessibility tree for a 6-digit
-- Apple two-factor code and prints the first match to stdout. If the browser
-- trust dialog is recognizable but only shows one button, the script clicks
-- it and keeps polling for the code dialog.
--
-- Requirements:
--   - The Apple 2FA dialog must be visible on this Mac.
--   - The caller (Terminal/Codex/etc.) must have Accessibility access.

property initialSettleDelaySeconds : 2
property postTrustClickDelaySeconds : 2
property trustDialogTextHints : {"browser", "this browser", "remember this browser", "remember browser", "do not ask again", "don't ask again"}
property codeEntryDialogTextHints : {"trusted devices", "verification code", "enter the verification code", "code sent"}

on run argv
	set timeoutSeconds to 60
	if (count of argv) > 0 then
		try
			set timeoutSeconds to (item 1 of argv) as integer
		on error
			error "invalid timeout seconds: " & (item 1 of argv)
		end try
	end if

	delay initialSettleDelaySeconds

	set deadlineAt to (current date)
	set deadlineAt to deadlineAt + timeoutSeconds
	repeat while (current date) is less than deadlineAt
		set code to my findTwoFactorCode()
		if code is not "" then
			return code
		end if
		delay 1
	end repeat

	error "timed out waiting for a 2FA code in FollowUpUI. Make sure the Apple dialog is visible and Accessibility access is enabled for your terminal."
end run

on findTwoFactorCode()
	try
		tell application "System Events"
			if not (exists process "FollowUpUI") then
				return ""
			end if

			tell process "FollowUpUI"
				repeat with currentWindow in windows
					set code to my scanWindowForCode(currentWindow)
					if code is not "" then
						my clickDoneButtonIfPresent(currentWindow)
						return code
					end if

					set didAdvanceTrustPrompt to my clickTrustButtonIfPresent(currentWindow)
					if didAdvanceTrustPrompt then
						delay postTrustClickDelaySeconds
					end if
				end repeat
			end tell
		end tell
	on error errMsg number errNum
		error "unable to inspect FollowUpUI via Accessibility: " & errMsg number errNum
	end try

	return ""
end findTwoFactorCode

on scanWindowForCode(theWindow)
	set code to my scanElement(theWindow)
	if code is not "" then
		return code
	end if

	try
		tell application "System Events" to set windowElements to entire contents of theWindow
		repeat with currentElement in windowElements
			set code to my scanElement(currentElement)
			if code is not "" then
				return code
			end if
		end repeat
	end try

	return ""
end scanWindowForCode

on clickTrustButtonIfPresent(theWindow)
	if my clickAllowButtonIfPresent(theWindow) then
		return true
	end if

	if not (my looksLikeTrustDialog(theWindow)) then
		return false
	end if

	return my clickRightmostButton(theWindow)
end clickTrustButtonIfPresent

on clickDoneButtonIfPresent(theWindow)
	return my clickDoneLabelIfPresent(theWindow)
end clickDoneButtonIfPresent

on clickAllowButtonIfPresent(theWindow)
	try
		tell application "System Events" to click button "Allow" of theWindow
		return true
	end try

	try
		tell application "System Events" to click button "Erlauben" of theWindow
		return true
	end try

	return false
end clickAllowButtonIfPresent

on clickDoneLabelIfPresent(theWindow)
	try
		tell application "System Events" to click button "Done" of theWindow
		return true
	end try

	try
		tell application "System Events" to click button "Fertig" of theWindow
		return true
	end try

	return false
end clickDoneLabelIfPresent

on clickRightmostButton(theWindow)
	try
		tell theWindow
			set windowButtons to buttons
		end tell
	on error
		return false
	end try

	if (count of windowButtons) = 0 then
		return false
	end if

	set chosenButton to missing value
	set chosenX to -1

	repeat with currentButton in windowButtons
		try
			set buttonPosition to position of currentButton
			set buttonX to item 1 of buttonPosition
			if chosenButton is missing value or buttonX > chosenX then
				set chosenButton to currentButton
				set chosenX to buttonX
			end if
		on error
			if chosenButton is missing value then
				set chosenButton to currentButton
			end if
		end try
	end repeat

	if chosenButton is missing value then
		return false
	end if

	return my pressButton(chosenButton)
end clickRightmostButton

on pressButton(buttonReference)
	try
		tell application "System Events" to perform action "AXPress" of buttonReference
		return true
	on error
		try
			tell application "System Events" to click buttonReference
			return true
		on error
			return false
		end try
	end try
end pressButton

on scanElement(theElement)
	set candidates to my elementTextCandidates(theElement)
	repeat with candidateText in candidates
		set code to my extractFirstCode(candidateText as text)
		if code is not "" then
			return code
		end if
	end repeat

	return ""
end scanElement

on looksLikeTrustDialog(theWindow)
	if my windowContainsAnyTextHint(theWindow, codeEntryDialogTextHints) then
		return false
	end if

	return my windowContainsAnyTextHint(theWindow, trustDialogTextHints)
end looksLikeTrustDialog

on windowContainsAnyTextHint(theWindow, textHints)
	if my elementContainsAnyTextHint(theWindow, textHints) then
		return true
	end if

	try
		tell application "System Events" to set windowElements to entire contents of theWindow
		repeat with currentElement in windowElements
			if my elementContainsAnyTextHint(currentElement, textHints) then
				return true
			end if
		end repeat
	end try

	return false
end windowContainsAnyTextHint

on elementContainsAnyTextHint(theElement, textHints)
	set candidates to my elementTextCandidates(theElement)
	repeat with candidateText in candidates
		if my textContainsAnyTextHint(candidateText as text, textHints) then
			return true
		end if
	end repeat

	return false
end elementContainsAnyTextHint

on elementTextCandidates(theElement)
	set candidates to {}

	try
		set end of candidates to my normalizeText(value of theElement)
	end try

	try
		set end of candidates to my normalizeText(name of theElement)
	end try

	try
		set end of candidates to my normalizeText(title of theElement)
	end try

	try
		set end of candidates to my normalizeText(description of theElement)
	end try

	return candidates
end elementTextCandidates

on textContainsAnyTextHint(candidateText, textHints)
	set candidateText to candidateText as text
	ignoring case
		repeat with currentHint in textHints
			if candidateText contains (currentHint as text) then
				return true
			end if
		end repeat
	end ignoring

	return false
end textContainsAnyTextHint

on normalizeText(candidateValue)
	if candidateValue is missing value then
		return ""
	end if

	if class of candidateValue is list then
		set previousDelimiters to AppleScript's text item delimiters
		set AppleScript's text item delimiters to " "
		set joinedText to candidateValue as text
		set AppleScript's text item delimiters to previousDelimiters
		return joinedText
	end if

	return candidateValue as text
end normalizeText

on extractFirstCode(sourceText)
	set sourceText to sourceText as text
	try
		set extractedCode to do shell script "/bin/echo " & quoted form of sourceText & " | /usr/bin/grep -Eo '(^|[^0-9])[0-9]{6}([^0-9]|$)' | /usr/bin/head -n1 | /usr/bin/tr -cd '0-9'"
	on error
		set extractedCode to ""
	end try
	if extractedCode is not "" then
		return extractedCode
	end if
	set extractedCode to my extractSpacedCodeText(sourceText)
	if extractedCode is not "" then
		return extractedCode
	end if
	set digitsOnly to my digitsOnlyText(sourceText)
	if (length of digitsOnly) is 6 then
		return digitsOnly
	end if
	return ""
end extractFirstCode

on extractSpacedCodeText(sourceText)
	set sourceText to sourceText as text
	try
		set extractedCode to do shell script "/bin/echo " & quoted form of sourceText & " | /usr/bin/grep -Eo '[0-9]([[:space:][:punct:]]+[0-9]){5}' | /usr/bin/head -n1 | /usr/bin/tr -cd '0-9'"
	on error
		set extractedCode to ""
	end try
	if (length of extractedCode) is 6 then
		return extractedCode
	end if
	return ""
end extractSpacedCodeText

on digitsOnlyText(sourceText)
	set sourceText to sourceText as text
	set extractedDigits to ""
	repeat with currentCharacter in characters of sourceText
		set currentCharacter to contents of currentCharacter
		if currentCharacter is in "0123456789" then
			set extractedDigits to extractedDigits & currentCharacter
		end if
	end repeat
	return extractedDigits
end digitsOnlyText
